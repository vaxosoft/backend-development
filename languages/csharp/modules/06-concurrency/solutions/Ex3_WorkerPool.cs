using System.Threading.Channels;

namespace M06.Exercises;

/// <summary>
/// A fixed pool of workers consuming a bounded queue of jobs. Workers start in the constructor.
/// </summary>
public sealed class Ex3_WorkerPool : IAsyncDisposable
{
    private readonly Channel<Func<CancellationToken, Task>> _queue;
    private readonly CancellationTokenSource _forceStop = new();
    private readonly Task[] _workers;
    private int _completed;
    private int _failed;
    private int _cancelled;

    public Ex3_WorkerPool(int workers, int queueCapacity)
    {
        _queue = Channel.CreateBounded<Func<CancellationToken, Task>>(queueCapacity);
        _workers = [.. Enumerable.Range(0, workers).Select(_ => Task.Run(WorkAsync))];
    }

    /// <summary>Jobs that finished without throwing.</summary>
    public int Completed => Volatile.Read(ref _completed);

    /// <summary>Jobs that threw (other than by cancellation). A failing job must not stop its worker.</summary>
    public int Failed => Volatile.Read(ref _failed);

    /// <summary>Jobs that ended with OperationCanceledException because the pool was forced to stop.</summary>
    public int Cancelled => Volatile.Read(ref _cancelled);

    /// <summary>Queue the job without waiting. False when the queue is full or the pool is shutting down.</summary>
    public bool TrySubmit(Func<CancellationToken, Task> job) => _queue.Writer.TryWrite(job);

    /// <summary>
    /// Stop accepting jobs, then wait up to <paramref name="drainTimeout"/> for queued and running jobs to finish.
    /// Returns true when everything drained. Otherwise cancel the token given to the jobs, wait for the workers
    /// to exit and return false. Calling it again is harmless.
    /// </summary>
    public async Task<bool> ShutdownAsync(TimeSpan drainTimeout)
    {
        _queue.Writer.TryComplete();
        var drained = Task.WhenAll(_workers);
        try
        {
            await drained.WaitAsync(drainTimeout);
            return true;
        }
        catch (TimeoutException)
        {
            await _forceStop.CancelAsync();
            await drained;
            return false;
        }
    }

    public async ValueTask DisposeAsync()
    {
        await ShutdownAsync(TimeSpan.Zero);
        _forceStop.Dispose();
    }

    private async Task WorkAsync()
    {
        await foreach (var job in _queue.Reader.ReadAllAsync())
        {
            try
            {
                await job(_forceStop.Token);
                Interlocked.Increment(ref _completed);
            }
            catch (OperationCanceledException)
            {
                Interlocked.Increment(ref _cancelled);
            }
            catch (Exception)
            {
                Interlocked.Increment(ref _failed);
            }
        }
    }
}
