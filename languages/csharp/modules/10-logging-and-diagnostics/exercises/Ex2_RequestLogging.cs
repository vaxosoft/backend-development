#pragma warning disable CS9113 // starter: the constructor parameters are for your implementation to use

using Microsoft.AspNetCore.Http;
using Microsoft.Extensions.Logging;

namespace M10.Exercises;

/// <summary>
/// Writes exactly one structured access-log entry per request.
/// <list type="bullet">
/// <item>Request id: use the <c>X-Request-ID</c> request header if present and non-empty, otherwise a new
/// <c>Guid.NewGuid().ToString("N")</c>. Put it on the response as <c>X-Request-ID</c> before calling the next middleware.</item>
/// <item>Open a logger scope with the template <c>"request {RequestId}"</c> around the whole request, so every line written by
/// later middleware or handlers carries the id.</item>
/// <item>After the pipeline finishes, log with the exact message template
/// <c>"HTTP {Method} {Path} responded {StatusCode} in {ElapsedMs} ms"</c> (never string interpolation).
/// <c>Path</c> is the path only: the query string may hold secrets and is not logged.
/// <c>ElapsedMs</c> is a double measured with <paramref name="time"/> (<c>GetTimestamp</c> / <c>GetElapsedTime</c>).</item>
/// <item>Level: Information below 400, Warning for 4xx, Error for 5xx.</item>
/// <item>If the pipeline throws: log StatusCode 500 at Error with the exception attached, then rethrow (do not swallow).</item>
/// </list>
/// </summary>
public sealed class RequestLoggingMiddleware(RequestDelegate next, ILogger<RequestLoggingMiddleware> logger, TimeProvider time)
{
    public Task InvokeAsync(HttpContext context)
    {
        throw new NotImplementedException();
    }
}
