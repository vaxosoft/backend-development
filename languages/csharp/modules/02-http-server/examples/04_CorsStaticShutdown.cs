using Microsoft.AspNetCore.Server.Kestrel.Core;
using Microsoft.Extensions.FileProviders;

namespace M02.Examples;

/// <summary>CORS, static files, server timeouts and graceful shutdown.</summary>
static class CorsStaticShutdown
{
    public static async Task<int> RunAsync(string[] args)
    {
        var builder = ExampleHost.CreateBuilder(args);

        builder.Services.AddCors(options => options.AddPolicy("frontend", policy => policy
            .WithOrigins("https://app.example.com")
            .WithMethods("GET", "POST")
            .WithHeaders("Content-Type")));

        builder.WebHost.ConfigureKestrel(kestrel =>
        {
            kestrel.AddServerHeader = false;
            kestrel.Limits.KeepAliveTimeout = TimeSpan.FromSeconds(30);
            kestrel.Limits.RequestHeadersTimeout = TimeSpan.FromSeconds(10);
            kestrel.Limits.MaxRequestBodySize = 1024 * 1024;
            kestrel.Limits.MinRequestBodyDataRate = new MinDataRate(bytesPerSecond: 240, gracePeriod: TimeSpan.FromSeconds(5));
        });

        builder.Services.Configure<HostOptions>(o => o.ShutdownTimeout = TimeSpan.FromSeconds(10));

        var app = builder.Build();

        app.UseStaticFiles(new StaticFileOptions
        {
            FileProvider = new PhysicalFileProvider(Path.Combine(AppContext.BaseDirectory, "public")),
            RequestPath = "/static",
        });
        app.UseCors("frontend");

        var lifetime = app.Lifetime;
        lifetime.ApplicationStopping.Register(() => Console.WriteLine("stopping: no new connections, draining requests"));
        lifetime.ApplicationStopped.Register(() => Console.WriteLine("stopped"));

        app.MapGet("/api/ping", () => "pong");

        // The request token is cancelled when the client disconnects, not when the host stops.
        app.MapGet("/slow", async (int ms, CancellationToken requestAborted) =>
        {
            await Task.Delay(Math.Clamp(ms, 0, 5000), requestAborted);
            return "done";
        });

        app.MapPost("/shutdown", (IHostApplicationLifetime host) =>
        {
            host.StopApplication();
            return Results.Accepted();
        });

        await app.RunAsync();
        return 0;
    }
}
