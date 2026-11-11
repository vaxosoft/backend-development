using System.Collections.Concurrent;
using System.Threading.Channels;

namespace M06.Examples;

public sealed record Job(int Id, string Type, string Payload);

public sealed record JobResult(int Id, string Status, string? Output, string? Error);

/// <summary>Fixed pool of workers reading a bounded queue; a failing job never kills a worker.</summary>
public sealed class WorkerPool(int workers, int capacity)
{
    private readonly Channel<Job> _queue = Channel.CreateBounded<Job>(capacity);
    private readonly ConcurrentDictionary<int, JobResult> _results = new();
    private Task[] _running = [];

    public IReadOnlyDictionary<int, JobResult> Results => _results;

    /// <summary>False when the queue is full (the HTTP layer would answer 429) or the pool is stopped.</summary>
    public bool TrySubmit(Job job) => _queue.Writer.TryWrite(job);

    public ValueTask SubmitAsync(Job job, CancellationToken ct = default) => _queue.Writer.WriteAsync(job, ct);

    public void Start() =>
        _running = [.. Enumerable.Range(0, workers).Select(_ => Task.Run(WorkAsync))];

    /// <summary>Stop accepting, let queued and running jobs finish.</summary>
    public async Task StopAsync()
    {
        _queue.Writer.Complete();
        await Task.WhenAll(_running);
    }

    private async Task WorkAsync()
    {
        await foreach (var job in _queue.Reader.ReadAllAsync())
        {
            try
            {
                var output = await RunAsync(job);
                _results[job.Id] = new JobResult(job.Id, "succeeded", output, null);
            }
            catch (InvalidOperationException e)
            {
                _results[job.Id] = new JobResult(job.Id, "failed", null, e.Message);
            }
        }
    }

    private static async Task<string> RunAsync(Job job)
    {
        await Task.Delay(2);
        return job.Type switch
        {
            "reverse" => new string(job.Payload.Reverse().ToArray()),
            "upper" => job.Payload.ToUpperInvariant(),
            _ => throw new InvalidOperationException($"unknown job type '{job.Type}'"),
        };
    }
}

/// <summary>Task Tracker flavour: a background job pool with a bounded queue and graceful shutdown.</summary>
public static class TrackerWorkerPool
{
    public static async Task<int> RunAsync(string[] args)
    {
        var pool = new WorkerPool(workers: 3, capacity: 2);

        // Nothing is consuming yet, so only 2 fit; the rest are refused immediately.
        Job[] jobs =
        [
            new(1, "reverse", "tracker"),
            new(2, "upper", "write docs"),
            new(3, "reverse", "stressed"),
            new(4, "explode", "?"),
            new(5, "upper", "done"),
        ];
        var refused = new List<Job>();
        foreach (var job in jobs)
        {
            var accepted = pool.TrySubmit(job);
            Console.WriteLine($"submit {job.Id}: {(accepted ? "accepted" : "queue full")}");
            if (!accepted)
            {
                refused.Add(job);
            }
        }

        pool.Start();

        // Now the workers drain the queue, so waiting for room succeeds.
        foreach (var job in refused)
        {
            await pool.SubmitAsync(job);
        }

        await pool.StopAsync();

        foreach (var result in pool.Results.Values.OrderBy(r => r.Id))
        {
            Console.WriteLine($"job {result.Id}: {result.Status} {result.Output ?? result.Error}");
        }

        Console.WriteLine($"submit after stop: {(pool.TrySubmit(new Job(6, "upper", "late")) ? "accepted" : "refused")}");
        return 0;
    }
}
