using System.Collections.Concurrent;

namespace M06.Examples;

/// <summary>Lost updates, then lock, Interlocked, ConcurrentDictionary and an async-safe critical section.</summary>
public static class SharedState
{
    private const int Workers = 8;
    private const int PerWorker = 100_000;
    private const int Expected = Workers * PerWorker;

    public static async Task<int> RunAsync(string[] args)
    {
        var racy = 0;
        await RunWorkers(_ => racy++);
        Console.WriteLine($"plain int++        : never more than {Expected} ({racy <= Expected}); usually fewer");

        var locked = 0;
        var gate = new object();
        await RunWorkers(_ =>
        {
            lock (gate)
            {
                locked++;
            }
        });
        Console.WriteLine($"lock               : {locked}");

        var atomic = 0;
        await RunWorkers(_ => Interlocked.Increment(ref atomic));
        Console.WriteLine($"Interlocked        : {atomic}");

        var perStatus = new ConcurrentDictionary<string, int>();
        string[] statuses = ["todo", "in_progress", "done"];
        await RunWorkers(worker => perStatus.AddOrUpdate(statuses[worker % 3], 1, (_, n) => n + 1));
        Console.WriteLine("ConcurrentDictionary: " + string.Join(", ", perStatus.OrderBy(kv => kv.Key).Select(kv => $"{kv.Key}={kv.Value}")));

        // lock cannot span an await (compile error CS1996); use SemaphoreSlim for an async critical section.
        var semaphore = new SemaphoreSlim(1, 1);
        var guarded = 0;
        await Task.WhenAll(Enumerable.Range(0, Workers).Select(async _ =>
        {
            for (var i = 0; i < 100; i++)
            {
                await semaphore.WaitAsync();
                try
                {
                    var snapshot = guarded;
                    await Task.Yield();
                    guarded = snapshot + 1;
                }
                finally
                {
                    semaphore.Release();
                }
            }
        }));
        Console.WriteLine($"SemaphoreSlim(1,1) : {guarded}");
        return 0;
    }

    private static Task RunWorkers(Action<int> step) =>
        Task.WhenAll(Enumerable.Range(0, Workers).Select(worker => Task.Run(() =>
        {
            for (var i = 0; i < PerWorker; i++)
            {
                step(worker);
            }
        })));
}
