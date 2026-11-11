using System.Net;

namespace P1.HelloHttp.Tests;

/// <summary>Runs the real Kestrel server on a loopback port to check graceful shutdown.</summary>
public class GracefulShutdownTests
{
    private static readonly TimeSpan Patience = TimeSpan.FromSeconds(5);

    private static async Task<(Task Server, CancellationTokenSource Stop, string Url)> StartAsync()
    {
        var stop = new CancellationTokenSource();
        var listening = new TaskCompletionSource<string>(TaskCreationOptions.RunContinuationsAsynchronously);
        var server = HelloApp.RunAsync("127.0.0.1:0", stop.Token, new CapturingSink(), listening.SetResult);
        var url = await listening.Task.WaitAsync(Patience);
        return (server, stop, url);
    }

    [Fact]
    public async Task Shutdown_LetsAnInFlightSlowRequestFinish()
    {
        var (server, stop, url) = await StartAsync();
        using var client = new HttpClient { BaseAddress = new Uri(url) };
        await client.GetStringAsync("/healthz"); // warm up the connection

        var slow = client.GetAsync("/slow?ms=600");
        await Task.Delay(200); // let the request reach the handler
        await stop.CancelAsync();

        var response = await slow.WaitAsync(Patience);
        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.Equal("done", await response.Content.ReadAsStringAsync());
        await server.WaitAsync(Patience);
    }

    [Fact]
    public async Task Shutdown_RefusesNewConnectionsAfterwards()
    {
        var (server, stop, url) = await StartAsync();

        await stop.CancelAsync();
        await server.WaitAsync(Patience);

        using var fresh = new HttpClient { BaseAddress = new Uri(url) };
        await Assert.ThrowsAsync<HttpRequestException>(() => fresh.GetAsync("/healthz"));
    }

    [Fact]
    public async Task Shutdown_ReturnsPromptlyWhenIdle()
    {
        var (server, stop, _) = await StartAsync();

        await stop.CancelAsync();

        await server.WaitAsync(Patience);
        Assert.True(server.IsCompletedSuccessfully);
    }
}
