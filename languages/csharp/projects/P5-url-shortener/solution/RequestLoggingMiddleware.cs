using System.Diagnostics;
using System.Text.Json;

namespace P5.UrlShortener;

/// <summary>
/// Writes one JSON line per request: <c>request_id</c>, <c>method</c>, <c>path</c>, <c>status</c>,
/// <c>duration_ms</c> and <c>user_id</c> when the caller is authenticated. Never logs bodies, headers
/// (except the request id) or the query string, so passwords and tokens cannot leak.
/// Also echoes the incoming <c>X-Request-ID</c> (or generates one) in the response header.
/// </summary>
public sealed class RequestLoggingMiddleware(RequestDelegate next, ILogLineSink sink)
{
    public const string RequestIdHeader = "X-Request-ID";
    private const int MaxRequestIdLength = 64;

    public async Task InvokeAsync(HttpContext context)
    {
        var started = Stopwatch.GetTimestamp();
        var requestId = context.Request.Headers[RequestIdHeader].FirstOrDefault();
        if (string.IsNullOrWhiteSpace(requestId) || requestId.Length > MaxRequestIdLength)
        {
            requestId = Guid.NewGuid().ToString("N");
        }

        context.Response.Headers[RequestIdHeader] = requestId;

        try
        {
            await next(context);
        }
        finally
        {
            var line = new Dictionary<string, object?>
            {
                ["request_id"] = requestId,
                ["method"] = context.Request.Method,
                ["path"] = context.Request.Path.Value,
                ["status"] = context.Response.StatusCode,
                ["duration_ms"] = Math.Round(Stopwatch.GetElapsedTime(started).TotalMilliseconds, 3),
            };

            // Authentication runs inside this middleware, so the user is only known after `next`.
            if (context.User.GetUserId() is { } userId)
            {
                line["user_id"] = userId;
            }

            sink.Write(JsonSerializer.Serialize(line));
        }
    }
}
