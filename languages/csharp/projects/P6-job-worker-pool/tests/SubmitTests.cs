using System.Net;
using System.Net.Http.Json;
using System.Text;
using Microsoft.Extensions.Time.Testing;

namespace P6.JobWorkerPool.Tests;

public class SubmitTests
{
    [Fact]
    public async Task Submit_returns_202_with_id_and_location()
    {
        await using var app = new PoolApp();
        var client = app.CreateClient();

        var response = await client.PostAsJsonAsync("/jobs", new { type = "reverse", payload = new { text = "abc" } });

        Assert.Equal(HttpStatusCode.Accepted, response.StatusCode);
        Assert.Equal("/jobs/1", response.Headers.Location?.OriginalString);
        Assert.Contains("\"id\":1", await response.Content.ReadAsStringAsync());
    }

    [Theory]
    [InlineData("""{"type":"teleport","payload":{}}""")]
    [InlineData("""{"payload":{}}""")]
    [InlineData("""{"type":"reverse","payload":{}}""")]
    [InlineData("""{"type":"reverse","payload":{"text":42}}""")]
    [InlineData("""{"type":"sleep","payload":{"ms":5001}}""")]
    [InlineData("""{"type":"sleep","payload":{"ms":-1}}""")]
    [InlineData("""{"type":"sleep","payload":{"ms":"soon"}}""")]
    public async Task Invalid_job_is_422_problem_details(string body)
    {
        await using var app = new PoolApp();
        var client = app.CreateClient();

        var response = await client.PostAsync("/jobs", new StringContent(body, Encoding.UTF8, "application/json"));

        Assert.Equal(HttpStatusCode.UnprocessableEntity, response.StatusCode);
        Assert.Equal("application/problem+json", response.Content.Headers.ContentType?.MediaType);
        Assert.Contains("validation_failed", await response.Content.ReadAsStringAsync());
    }

    [Fact]
    public async Task Malformed_json_is_400()
    {
        await using var app = new PoolApp();
        var client = app.CreateClient();

        var response = await client.PostAsync("/jobs", new StringContent("{not json", Encoding.UTF8, "application/json"));

        Assert.Equal(HttpStatusCode.BadRequest, response.StatusCode);
    }

    [Fact]
    public async Task Unknown_job_is_404_problem_details()
    {
        await using var app = new PoolApp();
        var client = app.CreateClient();

        var response = await client.GetAsync("/jobs/999");

        Assert.Equal(HttpStatusCode.NotFound, response.StatusCode);
        Assert.Contains("not_found", await response.Content.ReadAsStringAsync());
    }

    [Fact]
    public async Task List_returns_newest_job_first()
    {
        await using var app = new PoolApp();
        var client = app.CreateClient();
        var first = await app.SubmitAsync(client, "reverse", new { text = "one" });
        var second = await app.SubmitAsync(client, "reverse", new { text = "two" });
        var third = await app.SubmitAsync(client, "reverse", new { text = "three" });

        var list = await client.GetFromJsonAsync<JobListDto>("/jobs", PoolApp.Json);

        Assert.Equal([third, second, first], list!.Items.Select(j => j.Id));
        Assert.Equal(3, list.Total);
    }

    [Fact]
    public async Task Job_creation_time_comes_from_the_injected_clock()
    {
        var clock = new FakeTimeProvider(new DateTimeOffset(2030, 1, 1, 9, 0, 0, TimeSpan.Zero));
        await using var app = new PoolApp(time: clock);
        var client = app.CreateClient();

        var id = await app.SubmitAsync(client, "reverse", new { text = "abc" });
        var job = await PoolApp.WaitForFinishedAsync(client, id);

        Assert.Equal(clock.GetUtcNow(), job.CreatedAt);
    }
}
