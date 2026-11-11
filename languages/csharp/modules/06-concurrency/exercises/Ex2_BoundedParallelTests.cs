namespace M06.Exercises;

public class Ex2_BoundedParallelTests
{
    private static readonly TimeSpan Patience = TimeSpan.FromSeconds(5);

    [Fact]
    public async Task Results_keep_input_order_even_when_items_finish_out_of_order()
    {
        int[] items = [1, 2, 3, 4, 5, 6];

        var results = await Ex2_BoundedParallel.MapAsync(items, 3, async (n, ct) =>
        {
            await Task.Delay((7 - n) * 2, ct);
            return n * 10;
        });

        Assert.Equal([10, 20, 30, 40, 50, 60], results);
    }

    [Fact]
    public async Task Never_exceeds_the_limit_but_does_run_that_many_at_once()
    {
        var inFlight = 0;
        var peak = 0;
        var reachedLimit = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);

        await Ex2_BoundedParallel.MapAsync(Enumerable.Range(0, 9).ToList(), 3, async (_, ct) =>
        {
            var now = Interlocked.Increment(ref inFlight);
            lock (reachedLimit)
            {
                peak = Math.Max(peak, now);
            }

            if (now == 3)
            {
                reachedLimit.TrySetResult();
            }

            // Only returns if three calls really overlap: a sequential implementation times out here.
            await reachedLimit.Task.WaitAsync(Patience, ct);
            Interlocked.Decrement(ref inFlight);
            return 0;
        });

        Assert.Equal(3, peak);
    }

    [Fact]
    public async Task First_failure_stops_starting_new_items_and_rethrows_the_original_exception()
    {
        var started = 0;

        var ex = await Assert.ThrowsAsync<InvalidOperationException>(() =>
            Ex2_BoundedParallel.MapAsync(Enumerable.Range(0, 50).ToList(), 1, (n, _) =>
            {
                Interlocked.Increment(ref started);
                return n == 2 ? Task.FromException<int>(new InvalidOperationException("item 2")) : Task.FromResult(n);
            }));

        Assert.Equal("item 2", ex.Message);
        Assert.Equal(3, started);
    }

    [Fact]
    public async Task Failure_cancels_the_token_of_items_still_in_flight()
    {
        var sawCancellation = new TaskCompletionSource<bool>(TaskCreationOptions.RunContinuationsAsynchronously);
        var slowStarted = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);

        var run = Ex2_BoundedParallel.MapAsync([0, 1], 2, async (n, ct) =>
        {
            if (n == 0)
            {
                await slowStarted.Task;
                throw new InvalidOperationException("boom");
            }

            slowStarted.TrySetResult();
            try
            {
                await Task.Delay(Timeout.Infinite, ct);
            }
            catch (OperationCanceledException)
            {
                sawCancellation.TrySetResult(true);
                throw;
            }

            return n;
        });

        await Assert.ThrowsAsync<InvalidOperationException>(() => run.WaitAsync(Patience));
        Assert.True(await sawCancellation.Task.WaitAsync(Patience));
    }

    [Fact]
    public async Task Cancelling_the_caller_token_surfaces_as_OperationCanceledException()
    {
        using var cts = new CancellationTokenSource();
        var started = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);

        var run = Ex2_BoundedParallel.MapAsync([1, 2, 3], 2, async (n, ct) =>
        {
            started.TrySetResult();
            await Task.Delay(Timeout.Infinite, ct);
            return n;
        }, cts.Token);

        await started.Task.WaitAsync(Patience);
        await cts.CancelAsync();

        await Assert.ThrowsAnyAsync<OperationCanceledException>(() => run.WaitAsync(Patience));
    }

    [Fact]
    public async Task Rejects_a_limit_below_one()
    {
        await Assert.ThrowsAsync<ArgumentOutOfRangeException>(() =>
            Ex2_BoundedParallel.MapAsync([1], 0, (n, _) => Task.FromResult(n)));
    }

    [Fact]
    public async Task Empty_input_gives_empty_output()
    {
        Assert.Empty(await Ex2_BoundedParallel.MapAsync(Array.Empty<int>(), 4, (n, _) => Task.FromResult(n)));
    }
}
