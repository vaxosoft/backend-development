namespace M13.Examples;

public static class Timers
{
    public static async Task<int> RunAsync(string[] args)
    {
        var start = TimeProvider.System.GetTimestamp();

        // PeriodicTimer schedules ticks on a fixed cadence: work time does not add up as drift,
        // and a slow iteration never overlaps the next one.
        using var timer = new PeriodicTimer(TimeSpan.FromMilliseconds(50));
        for (var tick = 1; tick <= 3; tick++)
        {
            await timer.WaitForNextTickAsync();
            Console.WriteLine($"tick {tick}");
        }

        var elapsed = TimeProvider.System.GetElapsedTime(start);
        Console.WriteLine($"3 ticks took at least 140 ms: {elapsed >= TimeSpan.FromMilliseconds(140)}");

        using var cts = new CancellationTokenSource(TimeSpan.FromMilliseconds(120));
        var ticks = 0;
        try
        {
            while (await timer.WaitForNextTickAsync(cts.Token))
            {
                ticks++;
            }
        }
        catch (OperationCanceledException)
        {
            Console.WriteLine($"cancelled; ticks seen before cancellation: {ticks >= 1}");
        }

        using var stopped = new PeriodicTimer(TimeSpan.FromMilliseconds(10));
        stopped.Dispose();
        Console.WriteLine($"disposed timer yields: {await stopped.WaitForNextTickAsync()}");
        return 0;
    }
}
