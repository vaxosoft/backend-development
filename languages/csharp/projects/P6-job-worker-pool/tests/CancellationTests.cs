using System.Net;

namespace P6.JobWorkerPool.Tests;

public class CancellationTests
{
    [Fact]
    public async Task Deleting_a_queued_job_cancels_it_and_it_never_runs()
    {
        await using var app = new PoolApp(workers: 1);
        var client = app.CreateClient();
        var blocker = await app.SubmitAsync(client, "sleep", new { ms = 5000 });
        await PoolApp.WaitForStatusAsync(client, blocker, "running");
        var queued = await app.SubmitAsync(client, "reverse", new { text = "abc" });

        var response = await client.DeleteAsync($"/jobs/{queued}");
        await client.DeleteAsync($"/jobs/{blocker}");
        var follow = await app.SubmitAsync(client, "reverse", new { text = "xyz" });
        await PoolApp.WaitForFinishedAsync(client, follow);
        var job = await PoolApp.GetJobAsync(client, queued);

        Assert.Equal(HttpStatusCode.NoContent, response.StatusCode);
        Assert.Equal("cancelled", job.Status);
        Assert.Equal(0, job.Attempts);
    }

    [Fact]
    public async Task Deleting_a_running_job_cancels_it_and_frees_the_worker()
    {
        await using var app = new PoolApp(workers: 1);
        var client = app.CreateClient();
        var running = await app.SubmitAsync(client, "sleep", new { ms = 5000 });
        await PoolApp.WaitForStatusAsync(client, running, "running");
        var next = await app.SubmitAsync(client, "reverse", new { text = "abc" });

        var response = await client.DeleteAsync($"/jobs/{running}");
        var nextJob = await PoolApp.WaitForFinishedAsync(client, next);

        Assert.Equal(HttpStatusCode.NoContent, response.StatusCode);
        Assert.Equal("cancelled", (await PoolApp.GetJobAsync(client, running)).Status);
        Assert.Equal("succeeded", nextJob.Status);
    }

    [Fact]
    public async Task Deleting_a_finished_job_is_409()
    {
        await using var app = new PoolApp();
        var client = app.CreateClient();
        var id = await app.SubmitAsync(client, "reverse", new { text = "abc" });
        await PoolApp.WaitForFinishedAsync(client, id);

        var response = await client.DeleteAsync($"/jobs/{id}");

        Assert.Equal(HttpStatusCode.Conflict, response.StatusCode);
        Assert.Equal("succeeded", (await PoolApp.GetJobAsync(client, id)).Status);
    }

    [Fact]
    public async Task Deleting_an_unknown_job_is_404()
    {
        await using var app = new PoolApp();
        var client = app.CreateClient();

        var response = await client.DeleteAsync("/jobs/999");

        Assert.Equal(HttpStatusCode.NotFound, response.StatusCode);
    }
}
