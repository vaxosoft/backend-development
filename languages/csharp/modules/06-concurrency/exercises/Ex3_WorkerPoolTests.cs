namespace M06.Exercises;

public class Ex3_WorkerPoolTests
{
    private static readonly TimeSpan Patience = TimeSpan.FromSeconds(5);

    private static TaskCompletionSource NewGate() => new(TaskCreationOptions.RunContinuationsAsynchronously);

    [Fact]
    public async Task Runs_every_submitted_job_and_drains_on_shutdown()
    {
        await using var pool = new Ex3_WorkerPool(workers: 4, queueCapacity: 64);
        var ran = 0;

        for (var i = 0; i < 40; i++)
        {
            Assert.True(pool.TrySubmit(async _ =>
            {
                await Task.Delay(1);
                Interlocked.Increment(ref ran);
            }));
        }

        Assert.True(await pool.ShutdownAsync(Patience));
        Assert.Equal(40, ran);
        Assert.Equal(40, pool.Completed);
    }

    [Fact]
    public async Task Uses_all_workers_but_no_more()
    {
        await using var pool = new Ex3_WorkerPool(workers: 3, queueCapacity: 16);
        var inFlight = 0;
        var peak = 0;
        var allBusy = NewGate();

        for (var i = 0; i < 9; i++)
        {
            Assert.True(pool.TrySubmit(async ct =>
            {
                var now = Interlocked.Increment(ref inFlight);
                InterlockedMax(ref peak, now);
                if (now == 3)
                {
                    allBusy.TrySetResult();
                }

                await allBusy.Task.WaitAsync(Patience, ct);
                Interlocked.Decrement(ref inFlight);
            }));
        }

        Assert.True(await pool.ShutdownAsync(Patience));
        Assert.Equal(3, peak);
    }

    [Fact]
    public async Task Refuses_jobs_when_the_queue_is_full()
    {
        await using var pool = new Ex3_WorkerPool(workers: 1, queueCapacity: 1);
        var started = NewGate();
        var release = NewGate();

        Assert.True(pool.TrySubmit(async _ =>
        {
            started.SetResult();
            await release.Task;
        }));
        await started.Task.WaitAsync(Patience);

        Assert.True(pool.TrySubmit(_ => Task.CompletedTask));
        Assert.False(pool.TrySubmit(_ => Task.CompletedTask));

        release.SetResult();
        Assert.True(await pool.ShutdownAsync(Patience));
        Assert.Equal(2, pool.Completed);
    }

    [Fact]
    public async Task Refuses_jobs_after_shutdown_started()
    {
        await using var pool = new Ex3_WorkerPool(workers: 1, queueCapacity: 4);

        Assert.True(await pool.ShutdownAsync(Patience));

        Assert.False(pool.TrySubmit(_ => Task.CompletedTask));
        Assert.True(await pool.ShutdownAsync(Patience));
    }

    [Fact]
    public async Task A_failing_job_is_counted_and_the_worker_keeps_going()
    {
        await using var pool = new Ex3_WorkerPool(workers: 1, queueCapacity: 4);
        var after = 0;

        pool.TrySubmit(_ => throw new InvalidOperationException("boom"));
        pool.TrySubmit(_ => Task.FromException(new InvalidOperationException("async boom")));
        pool.TrySubmit(_ =>
        {
            after++;
            return Task.CompletedTask;
        });

        Assert.True(await pool.ShutdownAsync(Patience));
        Assert.Equal(1, after);
        Assert.Equal(2, pool.Failed);
        Assert.Equal(1, pool.Completed);
    }

    [Fact]
    public async Task Shutdown_that_cannot_drain_cancels_running_jobs()
    {
        await using var pool = new Ex3_WorkerPool(workers: 1, queueCapacity: 2);
        var started = NewGate();
        var sawCancellation = false;

        pool.TrySubmit(async ct =>
        {
            started.SetResult();
            try
            {
                await Task.Delay(Timeout.Infinite, ct);
            }
            catch (OperationCanceledException)
            {
                sawCancellation = true;
                throw;
            }
        });
        await started.Task.WaitAsync(Patience);

        var drained = await pool.ShutdownAsync(TimeSpan.FromMilliseconds(30)).WaitAsync(Patience);

        Assert.False(drained);
        Assert.True(sawCancellation);
        Assert.Equal(1, pool.Cancelled);
        Assert.Equal(0, pool.Failed);
    }

    private static void InterlockedMax(ref int target, int value)
    {
        int seen;
        while (value > (seen = Volatile.Read(ref target)) && Interlocked.CompareExchange(ref target, value, seen) != seen)
        {
        }
    }
}
