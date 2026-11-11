using System.Net;
using System.Net.Http.Json;

namespace P6.JobWorkerPool.Tests;

public class ExecutionTests
{
    [Fact]
    public async Task Reverse_job_succeeds_with_reversed_text()
    {
        await using var app = new PoolApp();
        var client = app.CreateClient();

        var id = await app.SubmitAsync(client, "reverse", new { text = "hello" });
        var job = await PoolApp.WaitForFinishedAsync(client, id);

        Assert.Equal("succeeded", job.Status);
        Assert.Equal("olleh", job.Result?.GetString());
        Assert.Equal(1, job.Attempts);
        Assert.Null(job.Error);
    }

    [Fact]
    public async Task Reverse_keeps_emoji_and_combining_marks_intact()
    {
        await using var app = new PoolApp();
        var client = app.CreateClient();

        var id = await app.SubmitAsync(client, "reverse", new { text = "a😀e\u0301b" });
        var job = await PoolApp.WaitForFinishedAsync(client, id);

        Assert.Equal("be\u0301😀a", job.Result?.GetString());
    }

    [Fact]
    public async Task Sleep_job_succeeds()
    {
        await using var app = new PoolApp();
        var client = app.CreateClient();

        var id = await app.SubmitAsync(client, "sleep", new { ms = 20 });
        var job = await PoolApp.WaitForFinishedAsync(client, id);

        Assert.Equal("succeeded", job.Status);
    }

    [Fact]
    public async Task Failing_job_is_retried_up_to_max_attempts_then_fails()
    {
        await using var app = new PoolApp(maxAttempts: 3);
        var client = app.CreateClient();

        var id = await app.SubmitAsync(client, "fail");
        var job = await PoolApp.WaitForFinishedAsync(client, id);

        Assert.Equal("failed", job.Status);
        Assert.Equal(3, job.Attempts);
        Assert.False(string.IsNullOrWhiteSpace(job.Error));
    }

    [Fact]
    public async Task Max_attempts_is_configurable()
    {
        await using var app = new PoolApp(maxAttempts: 1);
        var client = app.CreateClient();

        var id = await app.SubmitAsync(client, "fail");
        var job = await PoolApp.WaitForFinishedAsync(client, id);

        Assert.Equal("failed", job.Status);
        Assert.Equal(1, job.Attempts);
    }

    [Fact]
    public async Task Job_that_exceeds_the_timeout_fails_with_a_timeout_error()
    {
        await using var app = new PoolApp(maxAttempts: 2, jobTimeoutMs: 30);
        var client = app.CreateClient();

        var id = await app.SubmitAsync(client, "sleep", new { ms = 5000 });
        var job = await PoolApp.WaitForFinishedAsync(client, id);

        Assert.Equal("failed", job.Status);
        Assert.Equal(2, job.Attempts);
        Assert.Contains("timed out", job.Error);
    }

    [Fact]
    public async Task Full_queue_returns_429_with_retry_after()
    {
        await using var app = new PoolApp(workers: 1, queueCapacity: 1);
        var client = app.CreateClient();
        var running = await app.SubmitAsync(client, "sleep", new { ms = 5000 });
        await PoolApp.WaitForStatusAsync(client, running, "running");
        await app.SubmitAsync(client, "sleep", new { ms = 5000 });

        var response = await client.PostAsJsonAsync("/jobs", new { type = "reverse", payload = new { text = "x" } });

        Assert.Equal(HttpStatusCode.TooManyRequests, response.StatusCode);
        Assert.True(response.Headers.Contains("Retry-After"));
        Assert.Contains("rate_limited", await response.Content.ReadAsStringAsync());
    }

    [Fact]
    public async Task Pool_runs_exactly_as_many_jobs_in_parallel_as_it_has_workers()
    {
        await using var app = new PoolApp(workers: 2);
        var client = app.CreateClient();
        var ids = new List<int>();
        for (var i = 0; i < 3; i++)
        {
            ids.Add(await app.SubmitAsync(client, "sleep", new { ms = 5000 }));
        }

        await PoolApp.WaitForStatusAsync(client, ids[0], "running");
        await PoolApp.WaitForStatusAsync(client, ids[1], "running");
        await Task.Delay(50);

        Assert.Equal("queued", (await PoolApp.GetJobAsync(client, ids[2])).Status);
    }
}
