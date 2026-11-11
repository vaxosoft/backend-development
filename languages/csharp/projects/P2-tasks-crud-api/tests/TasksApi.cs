using System.Net;
using System.Net.Http.Json;
using System.Text;
using System.Text.Json;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Mvc.Testing;
using Microsoft.AspNetCore.TestHost;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Time.Testing;

namespace P2.TasksCrudApi.Tests;

/// <summary>An app with its own in-memory SQLite database and a clock the test controls.</summary>
public sealed class TasksApi : WebApplicationFactory<Program>
{
    public FakeTimeProvider Clock { get; } = new(new DateTimeOffset(2030, 1, 1, 9, 0, 0, TimeSpan.Zero));

    protected override void ConfigureWebHost(IWebHostBuilder builder)
    {
        var connectionString = $"Data Source=file:p2-{Guid.NewGuid():N}?mode=memory&cache=shared";
        builder.ConfigureAppConfiguration((_, config) =>
            config.AddInMemoryCollection(new Dictionary<string, string?> { ["ConnectionStrings:Tasks"] = connectionString }));
        builder.ConfigureTestServices(services => services.AddSingleton<TimeProvider>(Clock));
    }

    public Task<HttpResponseMessage> SendJsonAsync(HttpMethod method, string url, string json)
    {
        var request = new HttpRequestMessage(method, url)
        {
            Content = new StringContent(json, Encoding.UTF8, "application/json"),
        };
        return CreateClient().SendAsync(request);
    }

    public Task<HttpResponseMessage> PostAsync(string json) => SendJsonAsync(HttpMethod.Post, "/tasks", json);

    /// <summary>Creates a task and returns its id.</summary>
    public async Task<long> CreateAsync(string json)
    {
        var response = await PostAsync(json);
        Assert.Equal(HttpStatusCode.Created, response.StatusCode);
        return (await Body(response)).GetProperty("id").GetInt64();
    }

    public static async Task<JsonElement> Body(HttpResponseMessage response) =>
        (await response.Content.ReadFromJsonAsync<JsonElement>()).Clone();

    public async Task<JsonElement> ListAsync(string query = "") =>
        await Body(await CreateClient().GetAsync("/tasks" + query));

    public async Task<string[]> TitlesAsync(string query = "") =>
        [.. (await ListAsync(query)).GetProperty("items").EnumerateArray().Select(t => t.GetProperty("title").GetString()!)];
}
