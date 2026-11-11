using Microsoft.Extensions.Time.Testing;
using M12.TaskApi;

namespace M12.Examples;

public static class FakeTime
{
    /// <summary>Expires a fixed time after creation, judged by an injected clock.</summary>
    private sealed class Expiry(TimeSpan ttl, TimeProvider time)
    {
        private readonly DateTimeOffset _expiresAt = time.GetUtcNow() + ttl;

        public bool IsAlive => time.GetUtcNow() < _expiresAt;
    }

    public static async Task<int> RunAsync(string[] args)
    {
        var clock = new FakeTimeProvider(new DateTimeOffset(2030, 1, 1, 9, 0, 0, TimeSpan.Zero));

        using var store = new SqliteTaskStore();
        var task = await store.AddAsync(new NewTask("Write docs", null), clock.GetUtcNow());
        Console.WriteLine($"created_at is exactly {task.CreatedAt:O}");

        var session = new Expiry(TimeSpan.FromMinutes(10), clock);
        Console.WriteLine($"at +0m   session alive: {session.IsAlive}");
        clock.Advance(TimeSpan.FromMinutes(9));
        Console.WriteLine($"at +9m   session alive: {session.IsAlive}");
        clock.Advance(TimeSpan.FromMinutes(2));
        Console.WriteLine($"at +11m  session alive: {session.IsAlive}");

        var ticks = 0;
        using var timer = clock.CreateTimer(_ => ticks++, null, TimeSpan.FromSeconds(1), TimeSpan.FromSeconds(1));
        clock.Advance(TimeSpan.FromSeconds(3.5));
        Console.WriteLine($"timer ticks after 3.5 virtual seconds: {ticks}");

        var delay = Task.Delay(TimeSpan.FromHours(1), clock);
        Console.WriteLine($"one-hour delay completed before advancing: {delay.IsCompleted}");
        clock.Advance(TimeSpan.FromHours(1));
        await delay;
        Console.WriteLine($"one-hour delay completed after advancing: {delay.IsCompleted}");
        return 0;
    }
}
