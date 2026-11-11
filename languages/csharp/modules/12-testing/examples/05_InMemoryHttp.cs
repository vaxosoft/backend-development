using System.Net.Http.Json;
using Microsoft.AspNetCore.Mvc.Testing;
using Microsoft.AspNetCore.TestHost;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Time.Testing;

namespace M12.Examples;

public static class InMemoryHttp
{
    public static async Task<int> RunAsync(string[] args)
    {
        // WebApplicationFactory finds the app's content root through MvcTestingAppManifest.json in the working
        // directory; `dotnet test` runs there already, a console example has to move there.
        Directory.SetCurrentDirectory(AppContext.BaseDirectory);

        var clock = new FakeTimeProvider(new DateTimeOffset(2030, 1, 1, 9, 0, 0, TimeSpan.Zero));

        await using var factory = new WebApplicationFactory<Program>().WithWebHostBuilder(host =>
        {
            host.ConfigureLogging(logging => logging.ClearProviders());
            host.ConfigureTestServices(services => services.AddSingleton<TimeProvider>(clock));
        });
        using var client = factory.CreateClient();

        var created = await client.PostAsJsonAsync("/tasks", new { title = "  Write docs ", due_date = "2030-01-31" });
        Console.WriteLine($"POST /tasks -> {(int)created.StatusCode} Location={created.Headers.Location}");
        Console.WriteLine(await created.Content.ReadAsStringAsync());

        var invalid = await client.PostAsJsonAsync("/tasks", new { title = "" });
        Console.WriteLine($"POST /tasks (empty title) -> {(int)invalid.StatusCode} {invalid.Content.Headers.ContentType?.MediaType}");

        Console.WriteLine($"GET /tasks/99 -> {(int)(await client.GetAsync("/tasks/99")).StatusCode}");
        Console.WriteLine($"GET /tasks -> {await client.GetStringAsync("/tasks")}");
        return 0;
    }
}
