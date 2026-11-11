using System.Collections.Concurrent;
using System.Diagnostics;
using Microsoft.Extensions.Diagnostics.HealthChecks;
using Microsoft.Extensions.Logging;

namespace M10.Examples;

public sealed record TaskDto(int Id, string Title, string Status);

/// <summary>Flipped by the app: readiness answers "may I receive traffic?", liveness answers "is the process alive?".</summary>
public sealed class ReadinessState
{
    public volatile bool Ready;
}

public sealed class ReadinessCheck(ReadinessState state) : IHealthCheck
{
    public Task<HealthCheckResult> CheckHealthAsync(HealthCheckContext context, CancellationToken cancellationToken = default) =>
        Task.FromResult(state.Ready ? HealthCheckResult.Healthy("ready") : HealthCheckResult.Unhealthy("starting"));
}

/// <summary>Counts requests per status code for /metrics.</summary>
public sealed class RequestCounters
{
    private readonly ConcurrentDictionary<int, long> _byStatus = new();

    public void Increment(int status) => _byStatus.AddOrUpdate(status, 1, (_, n) => n + 1);

    public string Render()
    {
        var lines = new List<string> { $"requests_total {_byStatus.Values.Sum()}" };
        lines.AddRange(_byStatus.OrderBy(p => p.Key).Select(p => $"requests_by_status{{status=\"{p.Key}\"}} {p.Value}"));
        return string.Join('\n', lines) + "\n";
    }
}

/// <summary>Takes or creates X-Request-ID, echoes it, opens a logging scope, and writes one access-log line per request.</summary>
public sealed class RequestLoggingMiddleware(RequestDelegate next, ILogger<RequestLoggingMiddleware> logger, RequestCounters counters)
{
    public const string RequestIdKey = "RequestId";

    public async Task InvokeAsync(HttpContext context)
    {
        var requestId = context.Request.Headers["X-Request-ID"].FirstOrDefault() is { Length: > 0 and <= 64 } incoming
            && incoming.All(c => char.IsAsciiLetterOrDigit(c) || c is '-' or '_')
            ? incoming
            : Guid.NewGuid().ToString("N");
        context.Items[RequestIdKey] = requestId;
        // The exception handler clears response headers, so set it when the response starts.
        context.Response.OnStarting(() =>
        {
            context.Response.Headers["X-Request-ID"] = requestId;
            return Task.CompletedTask;
        });

        using var scope = logger.BeginScope("request {RequestId}", requestId);
        var started = Stopwatch.GetTimestamp();
        try
        {
            await next(context);
        }
        finally
        {
            var status = context.Response.StatusCode;
            counters.Increment(status);
            logger.Log(
                status >= 500 ? LogLevel.Error : LogLevel.Information,
                "HTTP {Method} {Path} responded {StatusCode} in {DurationMs:0.0} ms",
                context.Request.Method, context.Request.Path.Value, status, Stopwatch.GetElapsedTime(started).TotalMilliseconds);
        }
    }
}

/// <summary>Example 5: a Task Tracker service with request logging, health endpoints, metrics and safe errors. Listens on ADDR.</summary>
public static class TaskTrackerDiagnostics
{
    public static async Task<int> RunAsync(string[] args)
    {
        var addr = Environment.GetEnvironmentVariable("ADDR") ?? "127.0.0.1:8080";
        var builder = WebApplication.CreateBuilder(args);
        builder.WebHost.UseUrls($"http://{addr}");
        builder.Logging.ClearProviders().AddJsonConsole(o =>
        {
            o.IncludeScopes = true;
            o.JsonWriterOptions = new() { Indented = false };
        });
        builder.Logging.Configure(o => o.ActivityTrackingOptions = ActivityTrackingOptions.None);
        builder.Logging.AddFilter("Microsoft.AspNetCore", LogLevel.Warning);

        builder.Services.AddSingleton<ReadinessState>();
        builder.Services.AddSingleton<RequestCounters>();
        builder.Services.AddExceptionHandler<ProblemExceptionHandler>();
        builder.Services.AddProblemDetails(o => o.CustomizeProblemDetails = c =>
        {
            c.ProblemDetails.Extensions.Remove("traceId"); // random per request; request_id is the stable correlation key here
            c.ProblemDetails.Extensions["request_id"] = c.HttpContext.Items[RequestLoggingMiddleware.RequestIdKey];
        });
        builder.Services.AddHealthChecks().AddCheck<ReadinessCheck>("ready", tags: ["ready"]);

        var app = builder.Build();
        var logger = app.Services.GetRequiredService<ILoggerFactory>().CreateLogger("TaskTracker");
        var readiness = app.Services.GetRequiredService<ReadinessState>();
        var counters = app.Services.GetRequiredService<RequestCounters>();
        TaskDto[] tasks = [new(1, "Write docs", "todo"), new(2, "Ship it", "in_progress")];

        app.UseMiddleware<RequestLoggingMiddleware>(); // outermost: also sees what the exception handler turns into a 500
        app.UseExceptionHandler();
        app.UseStatusCodePages();

        app.MapHealthChecks("/healthz", new() { Predicate = _ => false }); // liveness: no checks, just "the process answers"
        app.MapHealthChecks("/readyz", new() { Predicate = r => r.Tags.Contains("ready") });
        app.MapGet("/metrics", () => Results.Text(counters.Render(), "text/plain; version=0.0.4"));
        app.MapGet("/tasks", () => tasks);
        app.MapGet("/tasks/{id:int}", (int id) => tasks.FirstOrDefault(t => t.Id == id) ?? throw new TaskNotFoundException(id));
        app.MapGet("/boom", string () => throw new InvalidOperationException("disk quota exceeded writing /var/lib/tt/tasks.db"));
        app.MapPost("/admin/ready/{value:bool}", (bool value) =>
        {
            readiness.Ready = value;
            logger.LogInformation("Readiness changed to {Ready}", value);
            return Results.NoContent();
        });

        app.Lifetime.ApplicationStarted.Register(() =>
        {
            readiness.Ready = true;
            logger.LogInformation("Listening on {Urls}", string.Join(", ", app.Urls));
        });
        app.Lifetime.ApplicationStopping.Register(() =>
        {
            readiness.Ready = false;
            logger.LogInformation("Shutting down");
        });

        await app.RunAsync(); // returns after Ctrl-C / SIGTERM once in-flight requests finish
        return 0;
    }
}
