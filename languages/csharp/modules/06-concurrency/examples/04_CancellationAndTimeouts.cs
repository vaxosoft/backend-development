namespace M06.Examples;

/// <summary>CancellationToken, CancelAfter, linked tokens, WaitAsync timeouts and bounded parallelism.</summary>
public static class CancellationAndTimeouts
{
    public static async Task<int> RunAsync(string[] args)
    {
        // 1. A deadline: CancelAfter cancels the token, Task.Delay observes it and throws.
        using (var cts = new CancellationTokenSource())
        {
            cts.CancelAfter(TimeSpan.FromMilliseconds(20));
            try
            {
                await Task.Delay(TimeSpan.FromSeconds(30), cts.Token);
            }
            catch (OperationCanceledException)
            {
                Console.WriteLine("deadline: Task.Delay canceled");
            }
        }

        // 2. Linked tokens: cancel on EITHER the caller (client disconnect) OR our own timeout.
        using var caller = new CancellationTokenSource();
        using var linked = CancellationTokenSource.CreateLinkedTokenSource(caller.Token);
        linked.CancelAfter(TimeSpan.FromSeconds(30));
        var work = CountUntilCanceledAsync(linked.Token);
        await Task.Delay(10);
        await caller.CancelAsync();
        Console.WriteLine($"linked: worker stopped after cancel, canceled={await work}");

        // 3. WaitAsync(timeout) stops WAITING; it does not stop the work itself.
        var never = new TaskCompletionSource();
        try
        {
            await never.Task.WaitAsync(TimeSpan.FromMilliseconds(20));
        }
        catch (TimeoutException)
        {
            Console.WriteLine("WaitAsync: TimeoutException (the underlying task is still pending)");
        }

        // 4. Bounded parallelism: Parallel.ForEachAsync with MaxDegreeOfParallelism.
        var inFlight = 0;
        var maxInFlight = 0;
        var processed = 0;
        await Parallel.ForEachAsync(
            Enumerable.Range(1, 20),
            new ParallelOptions { MaxDegreeOfParallelism = 3 },
            async (_, ct) =>
            {
                var now = Interlocked.Increment(ref inFlight);
                InterlockedMax(ref maxInFlight, now);
                await Task.Delay(5, ct);
                Interlocked.Decrement(ref inFlight);
                Interlocked.Increment(ref processed);
            });
        Console.WriteLine($"Parallel.ForEachAsync: processed={processed}, max in flight <= 3: {maxInFlight <= 3}");

        // 5. The same limit with SemaphoreSlim (works for any list of tasks).
        using var gate = new SemaphoreSlim(2);
        var current = 0;
        var peak = 0;
        await Task.WhenAll(Enumerable.Range(1, 10).Select(async _ =>
        {
            await gate.WaitAsync();
            try
            {
                InterlockedMax(ref peak, Interlocked.Increment(ref current));
                await Task.Delay(5);
                Interlocked.Decrement(ref current);
            }
            finally
            {
                gate.Release();
            }
        }));
        Console.WriteLine($"SemaphoreSlim(2): peak <= 2: {peak <= 2}");
        return 0;
    }

    private static async Task<bool> CountUntilCanceledAsync(CancellationToken ct)
    {
        try
        {
            while (true)
            {
                ct.ThrowIfCancellationRequested();
                await Task.Delay(1, ct);
            }
        }
        catch (OperationCanceledException)
        {
            return true;
        }
    }

    private static void InterlockedMax(ref int target, int value)
    {
        int seen;
        while (value > (seen = Volatile.Read(ref target)) && Interlocked.CompareExchange(ref target, value, seen) != seen)
        {
        }
    }
}
