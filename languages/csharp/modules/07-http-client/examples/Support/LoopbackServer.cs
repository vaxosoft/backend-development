namespace M07.Examples;

/// <summary>
/// A throw-away upstream API on the loopback interface so every example runs offline.
/// Binds to <c>ADDR</c> (host:port) when set, otherwise to a free port on 127.0.0.1.
/// </summary>
public sealed class LoopbackServer : IAsyncDisposable
{
    private readonly WebApplication _app;

    private LoopbackServer(WebApplication app, Uri baseAddress)
    {
        _app = app;
        BaseAddress = baseAddress;
    }

    public Uri BaseAddress { get; }

    public static async Task<LoopbackServer> StartAsync(Action<WebApplication> map)
    {
        var addr = Environment.GetEnvironmentVariable("ADDR") ?? "127.0.0.1:0";
        var builder = WebApplication.CreateBuilder();
        builder.Logging.ClearProviders();
        builder.WebHost.UseUrls($"http://{addr}");
        builder.Services.ConfigureHttpJsonOptions(o => Json.Configure(o.SerializerOptions));
        var app = builder.Build();
        map(app);
        await app.StartAsync();
        return new LoopbackServer(app, new Uri(app.Urls.First()));
    }

    public async ValueTask DisposeAsync()
    {
        await _app.StopAsync();
        await _app.DisposeAsync();
    }
}
