using System.Diagnostics;

namespace P1.HelloHttp;

/// <summary>Echoes a sane incoming X-Request-ID or generates one; exposes it as TraceIdentifier.</summary>
public sealed class RequestIdMiddleware(RequestDelegate next)
{
    public const string HeaderName = "X-Request-ID";
    private const int MaxLength = 64;

    public async Task InvokeAsync(HttpContext context)
    {
        var incoming = context.Request.Headers[HeaderName].FirstOrDefault();
        var id = IsAcceptable(incoming) ? incoming! : Guid.NewGuid().ToString("N");

        context.TraceIdentifier = id;
        context.Response.Headers[HeaderName] = id;
        await next(context);
    }

    private static bool IsAcceptable(string? id) =>
        !string.IsNullOrEmpty(id)
        && id.Length <= MaxLength
        && id.All(c => char.IsAsciiLetterOrDigit(c) || c is '-' or '_' or '.');
}

/// <summary>Writes one <see cref="RequestLogEntry"/> per request, after the response status is known.</summary>
public sealed class RequestLoggingMiddleware(RequestDelegate next, IRequestLogSink sink)
{
    public async Task InvokeAsync(HttpContext context)
    {
        var started = Stopwatch.GetTimestamp();
        try
        {
            await next(context);
        }
        finally
        {
            sink.Write(new RequestLogEntry(
                context.Request.Method,
                context.Request.Path.Value ?? "/",
                context.Response.StatusCode,
                Stopwatch.GetElapsedTime(started),
                context.TraceIdentifier));
        }
    }
}

/// <summary>Turns unhandled exceptions into a 500 problem+json that leaks nothing.</summary>
public sealed class ExceptionHandlingMiddleware(RequestDelegate next, ILogger<ExceptionHandlingMiddleware> logger)
{
    public async Task InvokeAsync(HttpContext context)
    {
        try
        {
            await next(context);
        }
        catch (Exception ex) when (!context.RequestAborted.IsCancellationRequested && !context.Response.HasStarted)
        {
            logger.LogError(ex, "Unhandled exception for {Method} {Path}", context.Request.Method, context.Request.Path);

            context.Response.Clear();
            context.Response.Headers[RequestIdMiddleware.HeaderName] = context.TraceIdentifier;
            await Results.Problem(
                statusCode: StatusCodes.Status500InternalServerError,
                title: "An unexpected error occurred",
                extensions: new Dictionary<string, object?> { ["request_id"] = context.TraceIdentifier })
                .ExecuteAsync(context);
        }
    }
}
