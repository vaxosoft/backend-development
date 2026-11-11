using System.Diagnostics;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;

namespace P6.JobWorkerPool.Tests;

public class ShutdownTests
{
    [Fact]
    public async Task Shutdown_lets_running_and_queued_jobs_finish()
    {
        await using var app = new PoolApp(workers: 1, drainTimeoutMs: 5000);
        var client = app.CreateClient();
        var ids = new List<int>();
        for (var i = 0; i < 3; i++)
        {
            ids.Add(await app.SubmitAsync(client, "sleep", new { ms = 40 }));
        }

        var store = app.Services.GetRequiredService<JobStore>();
        await app.Services.GetRequiredService<IHost>().StopAsync();

        Assert.All(ids, id => Assert.Equal(JobStatus.Succeeded, store.Get(id)!.Snapshot().Status));
    }

    [Fact]
    public async Task Shutdown_cancels_jobs_that_outlive_the_drain_timeout()
    {
        await using var app = new PoolApp(workers: 1, drainTimeoutMs: 50);
        var client = app.CreateClient();
        var running = await app.SubmitAsync(client, "sleep", new { ms = 5000 });
        await PoolApp.WaitForStatusAsync(client, running, "running");
        var queued = await app.SubmitAsync(client, "sleep", new { ms = 5000 });

        var store = app.Services.GetRequiredService<JobStore>();
        var watch = Stopwatch.StartNew();
        await app.Services.GetRequiredService<IHost>().StopAsync();

        Assert.True(watch.Elapsed < TimeSpan.FromSeconds(3), $"shutdown took {watch.Elapsed}");
        Assert.Equal(JobStatus.Cancelled, store.Get(running)!.Snapshot().Status);
        Assert.Equal(JobStatus.Cancelled, store.Get(queued)!.Snapshot().Status);
    }

    [Fact]
    public async Task Queue_stops_accepting_jobs_once_shutdown_begins()
    {
        await using var app = new PoolApp(workers: 1, drainTimeoutMs: 300);
        var client = app.CreateClient();
        await app.SubmitAsync(client, "sleep", new { ms = 5000 });
        var jobs = app.Services.GetRequiredService<JobService>();

        var stopping = app.Services.GetRequiredService<IHost>().StopAsync();
        var outcome = SubmitOutcome.Accepted;
        for (var i = 0; i < 200 && outcome == SubmitOutcome.Accepted; i++)
        {
            outcome = jobs.TrySubmit("reverse", default).Outcome;
            await Task.Delay(5);
        }

        await stopping;
        Assert.Equal(SubmitOutcome.Closed, outcome);
    }
}
