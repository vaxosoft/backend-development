namespace P1.HelloHttp;

/// <summary>Echoes a sane incoming X-Request-ID or generates one; exposes it as TraceIdentifier.</summary>
public sealed class RequestIdMiddleware(RequestDelegate next)
{
    public const string HeaderName = "X-Request-ID";

    public async Task InvokeAsync(HttpContext context)
    {
        // TODO: reuse the incoming header when it is valid (<= 64 chars of [A-Za-z0-9._-]),
        // otherwise generate one. Set context.TraceIdentifier and the response header.
        await next(context);
    }
}

/// <summary>Writes one <see cref="RequestLogEntry"/> per request, after the response status is known.</summary>
public sealed class RequestLoggingMiddleware(RequestDelegate next, IRequestLogSink sink)
{
    public async Task InvokeAsync(HttpContext context)
    {
        // TODO: measure the duration and call sink.Write(...) once the request finished (even on exceptions).
        _ = sink;
        await next(context);
    }
}

/// <summary>Turns unhandled exceptions into a 500 problem+json that leaks nothing.</summary>
public sealed class ExceptionHandlingMiddleware(RequestDelegate next, ILogger<ExceptionHandlingMiddleware> logger)
{
    public async Task InvokeAsync(HttpContext context)
    {
        // TODO: catch exceptions from `next`, log them (with the exception), and answer 500
        // application/problem+json without the exception message. Skip client aborts.
        _ = logger;
        await next(context);
    }
}
