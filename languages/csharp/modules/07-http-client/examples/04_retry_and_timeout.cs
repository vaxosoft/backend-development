using System.Net;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Http.Resilience;
using Polly;

namespace M07.Examples;

/// <summary>Example 4: retries with backoff and per-attempt timeouts through the resilience pipeline.</summary>
public static class RetryAndTimeout
{
    public static async Task<int> RunAsync(string[] args)
    {
        var calls = new Dictionary<string, int>();
        int Count(string route) => calls[route] = calls.GetValueOrDefault(route) + 1;

        await using var server = await LoopbackServer.StartAsync(app =>
        {
            app.MapGet("/flaky", () => Count("/flaky") <= 2 ? Results.StatusCode(503) : Results.Ok("recovered"));
            app.MapGet("/bad-request", () =>
            {
                Count("/bad-request");
                return Results.BadRequest();
            });
            app.MapGet("/always-down", () =>
            {
                Count("/always-down");
                return Results.StatusCode(500);
            });
            app.MapGet("/hangs-once", async (CancellationToken ct) =>
            {
                if (Count("/hangs-once") == 1)
                {
                    await Task.Delay(TimeSpan.FromSeconds(30), ct);
                }

                return Results.Ok("second attempt was fast");
            });
            app.MapGet("/hangs", async (CancellationToken ct) =>
            {
                await Task.Delay(TimeSpan.FromSeconds(30), ct);
                return Results.Ok();
            });
        });

        var services = new ServiceCollection();
        services.AddHttpClient("upstream", c => c.BaseAddress = server.BaseAddress)
            .AddResilienceHandler("retry", pipeline =>
            {
                pipeline.AddRetry(new HttpRetryStrategyOptions
                {
                    MaxRetryAttempts = 2,
                    Delay = TimeSpan.FromMilliseconds(5),
                    BackoffType = DelayBackoffType.Exponential,
                    UseJitter = false,
                });
                pipeline.AddTimeout(TimeSpan.FromMilliseconds(200));
            });

        await using var provider = services.BuildServiceProvider();
        var http = provider.GetRequiredService<IHttpClientFactory>().CreateClient("upstream");

        foreach (var route in new[] { "/flaky", "/bad-request", "/always-down", "/hangs-once" })
        {
            using var response = await http.GetAsync(route);
            Console.WriteLine($"{route,-13} -> {(int)response.StatusCode} after {calls[route]} attempt(s)");
        }

        // A caller deadline is separate from retry policy: cancelling stops the whole pipeline.
        using var cts = new CancellationTokenSource(TimeSpan.FromMilliseconds(50));
        try
        {
            await http.GetAsync("/hangs", cts.Token);
        }
        catch (OperationCanceledException) when (cts.IsCancellationRequested)
        {
            Console.WriteLine("caller deadline -> OperationCanceledException");
        }

        return 0;
    }
}
