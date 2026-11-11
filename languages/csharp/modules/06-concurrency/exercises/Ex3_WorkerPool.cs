namespace M06.Exercises;

/// <summary>
/// A fixed pool of workers consuming a bounded queue of jobs. Workers start in the constructor.
/// </summary>
public sealed class Ex3_WorkerPool : IAsyncDisposable
{
    public Ex3_WorkerPool(int workers, int queueCapacity) => throw new NotImplementedException();

    /// <summary>Jobs that finished without throwing.</summary>
    public int Completed => throw new NotImplementedException();

    /// <summary>Jobs that threw (other than by cancellation). A failing job must not stop its worker.</summary>
    public int Failed => throw new NotImplementedException();

    /// <summary>Jobs that ended with OperationCanceledException because the pool was forced to stop.</summary>
    public int Cancelled => throw new NotImplementedException();

    /// <summary>Queue the job without waiting. False when the queue is full or the pool is shutting down.</summary>
    public bool TrySubmit(Func<CancellationToken, Task> job) => throw new NotImplementedException();

    /// <summary>
    /// Stop accepting jobs, then wait up to <paramref name="drainTimeout"/> for queued and running jobs to finish.
    /// Returns true when everything drained. Otherwise cancel the token given to the jobs, wait for the workers
    /// to exit and return false. Calling it again is harmless.
    /// </summary>
    public Task<bool> ShutdownAsync(TimeSpan drainTimeout) => throw new NotImplementedException();

    public ValueTask DisposeAsync() => ValueTask.CompletedTask;
}
