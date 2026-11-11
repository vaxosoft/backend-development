using System.Net;
using System.Net.Http.Json;
using System.Text.Json;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Mvc.Testing;
using Microsoft.AspNetCore.TestHost;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Time.Testing;

namespace P6.JobWorkerPool.Tests;

public sealed record JobDto(int Id, string Type, string Status, int Attempts, JsonElement? Result, string? Error, DateTimeOffset CreatedAt);

public sealed record JobListDto(List<JobDto> Items, int Total);

/// <summary>An in-process copy of the service with test-friendly settings (1 ms retry delay, short drain).</summary>
public sealed class PoolApp(
    int workers = 2,
    int queueCapacity = 16,
    int maxAttempts = 3,
    int jobTimeoutMs = 5000,
    int drainTimeoutMs = 200,
    TimeProvider? time = null) : WebApplicationFactory<Program>
{
    public static readonly JsonSerializerOptions Json = new(JsonSerializerDefaults.Web)
    {
        PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower,
    };

    protected override void ConfigureWebHost(IWebHostBuilder builder)
    {
        builder.UseSetting("Jobs:Workers", workers.ToString());
        builder.UseSetting("Jobs:QueueCapacity", queueCapacity.ToString());
        builder.UseSetting("Jobs:MaxAttempts", maxAttempts.ToString());
        builder.UseSetting("Jobs:RetryDelayMs", "1");
        builder.UseSetting("Jobs:JobTimeoutMs", jobTimeoutMs.ToString());
        builder.UseSetting("Jobs:DrainTimeoutMs", drainTimeoutMs.ToString());

        if (time is not null)
        {
            builder.ConfigureTestServices(services => services.AddSingleton(time));
        }
    }

    public async Task<int> SubmitAsync(HttpClient client, string type, object? payload = null)
    {
        var response = await client.PostAsJsonAsync("/jobs", new { type, payload }, Json);
        Assert.Equal(HttpStatusCode.Accepted, response.StatusCode);
        using var body = JsonDocument.Parse(await response.Content.ReadAsStringAsync());
        return body.RootElement.GetProperty("id").GetInt32();
    }

    public static async Task<JobDto> GetJobAsync(HttpClient client, int id) =>
        (await client.GetFromJsonAsync<JobDto>($"/jobs/{id}", Json))!;

    public static async Task<JobDto> WaitForAsync(HttpClient client, int id, Func<JobDto, bool> condition)
    {
        var deadline = DateTime.UtcNow.AddSeconds(3);
        while (true)
        {
            var job = await GetJobAsync(client, id);
            if (condition(job))
            {
                return job;
            }

            if (DateTime.UtcNow > deadline)
            {
                Assert.Fail($"Timed out waiting for job {id}; last state: {job.Status}, attempts {job.Attempts}");
            }

            await Task.Delay(5);
        }
    }

    public static Task<JobDto> WaitForStatusAsync(HttpClient client, int id, string status) =>
        WaitForAsync(client, id, j => j.Status == status);

    public static Task<JobDto> WaitForFinishedAsync(HttpClient client, int id) =>
        WaitForAsync(client, id, j => j.Status is "succeeded" or "failed" or "cancelled");
}
