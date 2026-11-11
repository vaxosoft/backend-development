namespace M06.Examples;

/// <summary>Task, await, WhenAll/WhenAny, exceptions, and where CPU-bound work goes.</summary>
public static class AsyncBasics
{
    public static async Task<int> RunAsync(string[] args)
    {
        // Start all three, then await: they run concurrently, and results keep the input order.
        var slow = FetchTitleAsync(1, delayMs: 30);
        var fast = FetchTitleAsync(2, delayMs: 1);
        var medium = FetchTitleAsync(3, delayMs: 10);
        string[] titles = await Task.WhenAll(slow, fast, medium);
        Console.WriteLine($"WhenAll keeps input order: {string.Join(", ", titles)}");

        // WhenAny: the first to finish wins (a hand-rolled timeout).
        var winner = await Task.WhenAny(FetchTitleAsync(4, delayMs: 200), Task.Delay(5).ContinueWith(_ => "timeout"));
        Console.WriteLine($"WhenAny winner: {await winner}");

        // An exception inside an async method is stored in the Task and rethrown at the await.
        var failing = FailAsync("boom");
        Console.WriteLine("created the failing task without throwing");
        try
        {
            await failing;
        }
        catch (InvalidOperationException e)
        {
            Console.WriteLine($"caught at await: {e.Message}");
        }

        // WhenAll with several failures: await rethrows the first, the Task keeps all of them.
        var all = Task.WhenAll(FailAsync("one"), FailAsync("two"));
        try
        {
            await all;
        }
        catch (InvalidOperationException e)
        {
            Console.WriteLine($"await threw: {e.Message}; all failures: {all.Exception!.InnerExceptions.Count}");
        }

        // CPU-bound work does not become cheaper with async; Task.Run moves it off the calling thread.
        var primes = await Task.Run(() => Enumerable.Range(2, 5000).Count(IsPrime));
        Console.WriteLine($"primes below 5002: {primes}");
        return 0;
    }

    private static async Task<string> FetchTitleAsync(int id, int delayMs)
    {
        await Task.Delay(delayMs);
        return $"task-{id}";
    }

    private static async Task FailAsync(string message)
    {
        await Task.Yield();
        throw new InvalidOperationException(message);
    }

    private static bool IsPrime(int n) => n >= 2 && Enumerable.Range(2, (int)Math.Sqrt(n) - 1).All(d => n % d != 0);
}
