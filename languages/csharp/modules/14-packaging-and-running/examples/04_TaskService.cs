using System.Text.Json;

namespace M14.Examples;

public static class TaskService
{
    private static readonly TaskDto[] Tasks =
    [
        new(1, "Write docs", "todo", "2030-01-31"),
        new(2, "Ship it", "in_progress", null),
    ];

    public static async Task<int> RunAsync(string[] args)
    {
        var settings = RuntimeFlags.Load(args);

        var builder = WebApplication.CreateSlimBuilder(args);
        builder.WebHost.UseUrls($"http://{settings.Addr}");
        builder.Services.Configure<HostOptions>(o => o.ShutdownTimeout = TimeSpan.FromSeconds(settings.ShutdownTimeoutSeconds));
        builder.Services.ConfigureHttpJsonOptions(o =>
        {
            // Naming is a property of the options, not of the plugged-in context: set it here as well.
            o.SerializerOptions.PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower;
            o.SerializerOptions.TypeInfoResolverChain.Insert(0, AppJson.Default);
        });
        builder.Logging.SetMinimumLevel(Enum.Parse<LogLevel>(settings.LogLevel, ignoreCase: true));

        var app = builder.Build();
        var version = BuildInfoDemo.InformationalVersion();

        app.MapGet("/healthz", () => new HealthDto("ok"));
        app.MapGet("/version", () => new VersionDto("task-service", version));
        app.MapGet("/tasks", () => Tasks);
        app.MapGet("/slow", async (int ms, CancellationToken ct) =>
        {
            await Task.Delay(Math.Clamp(ms, 0, 2000), ct);
            return "done";
        });

        app.Logger.LogInformation("task-service {Version} in {Environment} listening on {Addr}", version, settings.Environment, settings.Addr);
        await app.RunAsync();
        return 0;
    }
}
