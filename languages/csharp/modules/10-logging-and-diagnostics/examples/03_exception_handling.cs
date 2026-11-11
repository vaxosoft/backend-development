using Microsoft.AspNetCore.Diagnostics;
using Microsoft.AspNetCore.Mvc;
using Microsoft.Extensions.Logging;

namespace M10.Examples;

public sealed class TaskNotFoundException(int id) : Exception($"task {id} does not exist")
{
    public int TaskId { get; } = id;
}

/// <summary>Maps exceptions to RFC 9457 problem details; unexpected ones are logged and never leaked.</summary>
public sealed class ProblemExceptionHandler(IProblemDetailsService problems, ILogger<ProblemExceptionHandler> logger) : IExceptionHandler
{
    public async ValueTask<bool> TryHandleAsync(HttpContext context, Exception exception, CancellationToken cancellationToken)
    {
        var (status, title) = exception switch
        {
            TaskNotFoundException => (StatusCodes.Status404NotFound, "Task not found"),
            _ => (StatusCodes.Status500InternalServerError, "An unexpected error occurred"),
        };

        if (status >= 500)
        {
            logger.LogError(exception, "Unhandled exception for {Method} {Path}", context.Request.Method, context.Request.Path);
        }
        else
        {
            logger.LogWarning("Request failed with {Status}: {Reason}", status, exception.Message);
        }

        context.Response.StatusCode = status;
        return await problems.TryWriteAsync(new ProblemDetailsContext
        {
            HttpContext = context,
            Exception = exception,
            ProblemDetails = new ProblemDetails
            {
                Status = status,
                Title = title,
                Detail = status == 404 ? exception.Message : null, // 5xx detail stays in the log
            },
        });
    }
}

/// <summary>Example 3: exception-handling middleware turns any exception into a safe problem+json response.</summary>
public static class ExceptionHandling
{
    public static async Task<int> RunAsync(string[] args)
    {
        var capture = new CaptureLoggerProvider();
        var builder = WebApplication.CreateBuilder();
        builder.Logging.ClearProviders().AddProvider(capture);
        builder.WebHost.UseUrls($"http://{Environment.GetEnvironmentVariable("ADDR") ?? "127.0.0.1:0"}");
        builder.Services.AddExceptionHandler<ProblemExceptionHandler>();
        builder.Services.AddProblemDetails(options => options.CustomizeProblemDetails = context =>
        {
            context.ProblemDetails.Extensions.Remove("traceId"); // random per request: keep the demo output stable
            context.ProblemDetails.Extensions["request_id"] = context.HttpContext.Request.Headers["X-Request-ID"].ToString();
        });

        var app = builder.Build();
        app.UseExceptionHandler();
        app.UseStatusCodePages();
        app.MapGet("/tasks/{id:int}", (int id) => id == 1 ? Results.Ok(new { id, title = "Write docs" }) : throw new TaskNotFoundException(id));
        app.MapGet("/boom", string () => throw new InvalidOperationException("disk quota exceeded writing /var/lib/tt/tasks.db"));
        await app.StartAsync();

        using var http = new HttpClient { BaseAddress = new Uri(app.Urls.First()) };
        var n = 0;
        foreach (var path in new[] { "/tasks/1", "/tasks/9", "/boom", "/no-such-route" })
        {
            using var request = new HttpRequestMessage(HttpMethod.Get, path);
            request.Headers.Add("X-Request-ID", $"req-{++n}");
            using var response = await http.SendAsync(request);
            Console.WriteLine($"GET {path} -> {(int)response.StatusCode} {response.Content.Headers.ContentType}");
            Console.WriteLine($"  {await response.Content.ReadAsStringAsync()}");
        }

        await app.StopAsync();

        Console.WriteLine("server log:");
        foreach (var entry in capture.Entries.Where(e => e.Category.StartsWith("M10")))
        {
            Console.WriteLine($"  {entry.Level}: {entry.Message}{(entry.Exception is null ? "" : $" [{entry.Exception.GetType().Name}: {entry.Exception.Message}]")}");
        }

        return 0;
    }
}
