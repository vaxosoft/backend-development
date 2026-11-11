using System.Threading.Channels;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Time.Testing;

namespace M13.Examples;

public static class ReminderWorkerDemo
{
    private sealed record Job(string Title, DateOnly Due);

    /// <summary>Wakes up on a fixed period and reports each overdue task once.</summary>
    private sealed class ReminderWorker(TimeProvider time, Channel<string> output) : BackgroundService
    {
        private static readonly Job[] Jobs = [new("Write docs", new DateOnly(2030, 1, 2)), new("Ship", new DateOnly(2030, 1, 3))];

        protected override async Task ExecuteAsync(CancellationToken stoppingToken)
        {
            var notified = new HashSet<string>();
            using var timer = new PeriodicTimer(TimeSpan.FromHours(12), time);
            try
            {
                // Tell the caller the timer exists: a fake clock advanced earlier would never fire it.
                await output.Writer.WriteAsync("worker started", stoppingToken);
                while (await timer.WaitForNextTickAsync(stoppingToken))
                {
                    var now = time.GetUtcNow();
                    var today = DateOnly.FromDateTime(now.UtcDateTime);
                    var due = Jobs.Where(j => j.Due < today && notified.Add(j.Title)).Select(j => j.Title).ToList();
                    await output.Writer.WriteAsync($"{now:yyyy-MM-dd HH:mm}Z reminders: [{string.Join(", ", due)}]", stoppingToken);
                }
            }
            catch (OperationCanceledException)
            {
                // normal shutdown
            }

            await output.Writer.WriteAsync("worker stopped", CancellationToken.None);
        }
    }

    public static async Task<int> RunAsync(string[] args)
    {
        var clock = new FakeTimeProvider(new DateTimeOffset(2030, 1, 2, 22, 0, 0, TimeSpan.Zero));
        var output = Channel.CreateUnbounded<string>();

        var builder = Host.CreateApplicationBuilder(args);
        builder.Logging.ClearProviders();
        builder.Services.AddSingleton<TimeProvider>(clock);
        builder.Services.AddSingleton(output);
        builder.Services.AddHostedService<ReminderWorker>();
        using var host = builder.Build();

        await host.StartAsync();
        Console.WriteLine(await output.Reader.ReadAsync());
        for (var i = 0; i < 3; i++)
        {
            clock.Advance(TimeSpan.FromHours(12));
            Console.WriteLine(await output.Reader.ReadAsync());
        }

        await host.StopAsync();
        Console.WriteLine(await output.Reader.ReadAsync());
        return 0;
    }
}
