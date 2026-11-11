using System.Net;
using System.Net.Http.Json;
using System.Text;
using Microsoft.Extensions.DependencyInjection;
using NSubstitute;

namespace P7.TestedService.Tests;

public class BehaviourTests
{
    [Fact]
    public async Task Healthz_reports_ok_and_the_configured_environment()
    {
        await using var app = new ServiceApp(new Dictionary<string, string> { ["ENV"] = "prod" });
        var client = app.CreateClient();

        var response = await client.GetAsync("/healthz");

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.Equal("""{"status":"ok","env":"prod"}""", await response.Content.ReadAsStringAsync());
    }

    [Fact]
    public async Task Readyz_is_200_once_the_application_has_started()
    {
        await using var app = new ServiceApp();
        var client = app.CreateClient();

        var response = await client.GetAsync("/readyz");

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
    }

    [Fact]
    public async Task Readyz_is_503_while_the_service_is_not_ready()
    {
        await using var app = new ServiceApp();
        var client = app.CreateClient();
        app.Services.GetRequiredService<ReadinessState>().MarkNotReady();

        var response = await client.GetAsync("/readyz");

        Assert.Equal(HttpStatusCode.ServiceUnavailable, response.StatusCode);
        Assert.Contains("not_ready", await response.Content.ReadAsStringAsync());
    }

    [Fact]
    public async Task Metrics_counts_every_request_including_the_scrape_itself()
    {
        await using var app = new ServiceApp();
        var client = app.CreateClient();
        await client.GetAsync("/healthz");
        await client.GetAsync("/tasks");
        await client.GetAsync("/does-not-exist");

        var response = await client.GetAsync("/metrics");

        Assert.Equal("text/plain", response.Content.Headers.ContentType?.MediaType);
        Assert.Equal("requests_total 4\n", await response.Content.ReadAsStringAsync());
    }

    [Fact]
    public async Task Created_task_is_returned_with_201_and_location_and_can_be_fetched()
    {
        await using var app = new ServiceApp();
        var client = app.CreateClient();

        var created = await client.PostAsJsonAsync("/tasks", new { title = "  Write docs  " });
        var fetched = await client.GetAsync(created.Headers.Location);

        Assert.Equal(HttpStatusCode.Created, created.StatusCode);
        Assert.Equal("/tasks/1", created.Headers.Location?.OriginalString);
        Assert.Equal(HttpStatusCode.OK, fetched.StatusCode);
        var body = await fetched.Content.ReadAsStringAsync();
        Assert.Contains("\"title\":\"Write docs\"", body);
        Assert.Contains("\"status\":\"todo\"", body);
    }

    [Theory]
    [InlineData("""{"title":""}""")]
    [InlineData("""{"title":"   "}""")]
    [InlineData("""{}""")]
    public async Task Blank_title_is_422_problem_details(string body)
    {
        await using var app = new ServiceApp();
        var client = app.CreateClient();

        var response = await client.PostAsync("/tasks", new StringContent(body, Encoding.UTF8, "application/json"));

        Assert.Equal(HttpStatusCode.UnprocessableEntity, response.StatusCode);
        Assert.Equal("application/problem+json", response.Content.Headers.ContentType?.MediaType);
    }

    [Fact]
    public async Task Unknown_task_is_404()
    {
        await using var app = new ServiceApp();
        var client = app.CreateClient();

        var response = await client.GetAsync("/tasks/42");

        Assert.Equal(HttpStatusCode.NotFound, response.StatusCode);
    }

    [Fact]
    public async Task Task_list_comes_from_the_store_behind_the_interface()
    {
        var store = Substitute.For<ITaskStore>();
        store.List().Returns([new TaskItem(7, "From the fake", "done", DateTimeOffset.UnixEpoch)]);
        await using var app = new ServiceApp(configureServices: s => s.AddSingleton(store));
        var client = app.CreateClient();

        var body = await client.GetStringAsync("/tasks");

        Assert.Contains("\"id\":7", body);
        Assert.Contains("\"total\":1", body);
    }

    [Fact]
    public void Invalid_environment_makes_startup_fail()
    {
        using var app = new ServiceApp(new Dictionary<string, string> { ["PORT"] = "not-a-port" });

        Assert.Throws<InvalidOperationException>(() => app.CreateClient());
    }
}
