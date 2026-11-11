using System.Text.Json;
using System.Threading.Channels;
using Microsoft.Extensions.Options;

namespace P6.JobWorkerPool;

public enum SubmitOutcome
{
    Accepted,
    QueueFull,
    Closed,
}

public enum CancelOutcome
{
    Cancelled,
    NotFound,
    AlreadyFinished,
}

/// <summary>Owns the bounded queue. HTTP handlers write to it, <see cref="WorkerPool"/> reads from it.</summary>
public sealed class JobService(IOptions<JobsOptions> options, JobStore store, TimeProvider time)
{
    private readonly Channel<Job> _queue = Channel.CreateBounded<Job>(
        new BoundedChannelOptions(options.Value.QueueCapacity) { FullMode = BoundedChannelFullMode.Wait });

    private volatile bool _closed;

    public ChannelReader<Job> Reader => _queue.Reader;

    public (SubmitOutcome Outcome, Job? Job) TrySubmit(string type, JsonElement payload)
    {
        if (_closed)
        {
            return (SubmitOutcome.Closed, null);
        }

        var job = new Job(store.NextId(), type, payload, time.GetUtcNow());

        // TryWrite never waits: a full queue must answer 429 right away instead of blocking the request.
        if (!_queue.Writer.TryWrite(job))
        {
            return (_closed ? SubmitOutcome.Closed : SubmitOutcome.QueueFull, null);
        }

        store.Add(job);
        return (SubmitOutcome.Accepted, job);
    }

    public CancelOutcome Cancel(int id)
    {
        var job = store.Get(id);
        if (job is null)
        {
            return CancelOutcome.NotFound;
        }

        return job.TryCancel() ? CancelOutcome.Cancelled : CancelOutcome.AlreadyFinished;
    }

    /// <summary>Rejects new jobs; workers still drain what is already queued.</summary>
    public void StopAccepting()
    {
        _closed = true;
        _queue.Writer.TryComplete();
    }
}
