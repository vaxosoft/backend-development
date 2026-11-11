using Microsoft.Extensions.DependencyInjection.Extensions;

namespace P1.HelloHttp;

public static class HelloApp
{
    /// <summary>Builds the app: services, middleware pipeline and endpoints. Does not start it.</summary>
    public static WebApplication Create(string[] args, IRequestLogSink? sink = null)
    {
        var builder = WebApplication.CreateBuilder(args);
        ConfigureTimeouts(builder);

        builder.Services.TryAddSingleton(TimeProvider.System);
        if (sink is not null)
        {
            builder.Services.AddSingleton(sink);
        }
        else
        {
            builder.Services.AddSingleton<IRequestLogSink, LoggerRequestLogSink>();
        }

        var app = builder.Build();

        // Order matters: the request id must exist before logging, and the exception
        // handler sits inside the logger so the logged status is the 500 it produces.
        app.UseMiddleware<RequestIdMiddleware>();
        app.UseMiddleware<RequestLoggingMiddleware>();
        app.UseMiddleware<ExceptionHandlingMiddleware>();
        app.MapEndpoints();
        return app;
    }

    /// <summary>
    /// Serves on <paramref name="addr"/> (host:port, port 0 = any free port) until
    /// <paramref name="shutdown"/> is cancelled or the process gets SIGINT/SIGTERM.
    /// In-flight requests finish before the returned task completes.
    /// </summary>
    public static async Task RunAsync(
        string addr, CancellationToken shutdown, IRequestLogSink? sink = null, Action<string>? onListening = null)
    {
        await using var app = Create([], sink);
        app.Urls.Add($"http://{addr}");

        await app.StartAsync();
        onListening?.Invoke(app.Urls.First());

        // Returns on cancellation, SIGINT or SIGTERM; it then stops the host, which closes the
        // listeners at once and waits (up to ShutdownTimeout) for in-flight requests.
        await app.WaitForShutdownAsync(shutdown);
    }

    private static void ConfigureTimeouts(WebApplicationBuilder builder)
    {
        builder.WebHost.ConfigureKestrel(kestrel =>
        {
            kestrel.Limits.RequestHeadersTimeout = TimeSpan.FromSeconds(10);
            kestrel.Limits.KeepAliveTimeout = TimeSpan.FromSeconds(30);
            kestrel.Limits.MaxRequestBodySize = 64 * 1024;
        });
        builder.Services.Configure<HostOptions>(host => host.ShutdownTimeout = TimeSpan.FromSeconds(10));
    }
}
