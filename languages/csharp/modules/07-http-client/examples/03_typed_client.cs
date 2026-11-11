using System.Net.Http.Json;
using Microsoft.Extensions.DependencyInjection;

namespace M07.Examples;

/// <summary>Example 3: IHttpClientFactory, a typed client and a DelegatingHandler pipeline.</summary>
public static class TypedClient
{
    public static async Task<int> RunAsync(string[] args)
    {
        await using var server = await LoopbackServer.StartAsync(app =>
        {
            app.MapGet("/whoami", (HttpRequest request) => new
            {
                api_key = request.Headers["X-Api-Key"].ToString(),
                stamped_by = request.Headers["X-Stamped-By"].ToString(),
                user_agent = request.Headers.UserAgent.ToString(),
            });
        });

        var services = new ServiceCollection();
        services.AddTransient<StampHandler>();
        services.AddHttpClient<WhoAmIClient>(client =>
            {
                client.BaseAddress = server.BaseAddress;
                client.DefaultRequestHeaders.Add("X-Api-Key", "demo-key");
                client.DefaultRequestHeaders.UserAgent.ParseAdd("task-tracker-cli/1.0");
            })
            .AddHttpMessageHandler<StampHandler>()
            .SetHandlerLifetime(TimeSpan.FromMinutes(2));

        await using var provider = services.BuildServiceProvider();

        // Resolve the typed client wherever you need it; the factory pools and recycles the connections.
        for (var i = 1; i <= 2; i++)
        {
            await using var scope = provider.CreateAsyncScope();
            var client = scope.ServiceProvider.GetRequiredService<WhoAmIClient>();
            Console.WriteLine($"call {i}: {await client.WhoAmIAsync()}");
        }

        return 0;
    }

    private sealed class WhoAmIClient(HttpClient http)
    {
        public async Task<string> WhoAmIAsync(CancellationToken cancellationToken = default)
        {
            var info = await http.GetFromJsonAsync<Dictionary<string, string>>("whoami", cancellationToken);
            return string.Join(", ", info!.Select(pair => $"{pair.Key}={pair.Value}"));
        }
    }

    private sealed class StampHandler : DelegatingHandler
    {
        protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
        {
            request.Headers.Add("X-Stamped-By", nameof(StampHandler));
            return base.SendAsync(request, cancellationToken);
        }
    }
}
