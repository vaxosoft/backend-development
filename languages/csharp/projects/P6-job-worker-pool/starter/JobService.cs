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

        // TODO (submit): put the job on _queue WITHOUT waiting (Writer.TryWrite). When that fails the queue is full:
        // return QueueFull (or Closed if _closed became true meanwhile) and do not register the job.
        store.Add(job);
        return (SubmitOutcome.Accepted, job);
    }

    public CancelOutcome Cancel(int id)
    {
        // TODO (cancel): unknown id -> NotFound; otherwise ask the job to cancel itself (Job.TryCancel):
        // true -> Cancelled, false -> AlreadyFinished.
        return store.Get(id) is null ? CancelOutcome.NotFound : CancelOutcome.AlreadyFinished;
    }

    /// <summary>Rejects new jobs; workers still drain what is already queued.</summary>
    public void StopAccepting()
    {
        _closed = true;
        _queue.Writer.TryComplete();
    }
}
