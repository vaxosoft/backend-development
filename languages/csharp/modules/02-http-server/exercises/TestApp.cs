using Microsoft.AspNetCore.TestHost;
using Microsoft.Extensions.Logging;

namespace M02.Exercises;

/// <summary>An in-memory server (no sockets) plus a client wired to it.</summary>
sealed class TestApp : IAsyncDisposable
{
    readonly WebApplication _app;

    TestApp(WebApplication app)
    {
        _app = app;
        Client = app.GetTestClient();
    }

    public HttpClient Client { get; }

    public static async Task<TestApp> StartAsync(Action<WebApplication> configure)
    {
        var builder = WebApplication.CreateBuilder();
        builder.WebHost.UseTestServer();
        builder.Logging.ClearProviders();
        var app = builder.Build();
        configure(app);
        await app.StartAsync();
        return new TestApp(app);
    }

    public async ValueTask DisposeAsync()
    {
        Client.Dispose();
        await _app.DisposeAsync();
    }
}
