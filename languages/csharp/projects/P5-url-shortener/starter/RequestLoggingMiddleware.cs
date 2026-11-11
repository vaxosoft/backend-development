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

    public async Task InvokeAsync(HttpContext context)
    {
        // TODO(P5): assign/echo the request id, time the request, and write one JSON line via `sink`.
        _ = sink;
        await next(context);
    }
}
