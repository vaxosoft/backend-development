using System.Net;

namespace M07.Examples;

/// <summary>Example 1: a non-2xx response is not an exception until you ask for one.</summary>
public static class GetAndStatus
{
    public static async Task<int> RunAsync(string[] args)
    {
        await using var server = await LoopbackServer.StartAsync(app =>
        {
            app.MapGet("/hello", () => "hello from upstream");
            app.MapGet("/missing", () => Results.NotFound());
            app.MapGet("/boom", () => Results.StatusCode(503));
        });

        // One long-lived client; never `using` a client that other code shares.
        using var client = new HttpClient { BaseAddress = server.BaseAddress, Timeout = TimeSpan.FromSeconds(5) };

        using var ok = await client.GetAsync("hello");
        Console.WriteLine($"GET /hello   -> {(int)ok.StatusCode} {ok.ReasonPhrase}, success={ok.IsSuccessStatusCode}");
        Console.WriteLine($"content-type -> {ok.Content.Headers.ContentType}");
        Console.WriteLine($"body         -> {await ok.Content.ReadAsStringAsync()}");

        using var missing = await client.GetAsync("missing");
        Console.WriteLine($"GET /missing -> {(int)missing.StatusCode} {missing.ReasonPhrase}, success={missing.IsSuccessStatusCode}");

        try
        {
            using var boom = await client.GetAsync("boom");
            boom.EnsureSuccessStatusCode();
        }
        catch (HttpRequestException ex) when (ex.StatusCode is HttpStatusCode status)
        {
            Console.WriteLine($"GET /boom    -> EnsureSuccessStatusCode threw, StatusCode={(int)status}");
        }

        try
        {
            // The string/stream shortcuts call EnsureSuccessStatusCode for you.
            await client.GetStringAsync("missing");
        }
        catch (HttpRequestException ex)
        {
            Console.WriteLine($"GetStringAsync(/missing) threw, StatusCode={(int?)ex.StatusCode}");
        }

        return 0;
    }
}
