namespace M02.Exercises;

/// <summary>
/// Gives every request an id. An incoming X-Request-ID is reused when it is 1-64 characters of
/// letters, digits, '.', '_' or '-'; otherwise a new 32-character lowercase hex id is generated.
/// </summary>
public sealed class RequestIdMiddleware(RequestDelegate next)
{
    public const string HeaderName = "X-Request-ID";

    /// <summary>The id of the current request, available to later middleware and handlers.</summary>
    public static string Get(HttpContext context) => (string)context.Items[HeaderName]!;

    /// <summary>
    /// Stores the id in <c>context.Items[HeaderName]</c>, calls the next middleware, and makes sure the
    /// same id is on the response (also on error responses).
    /// </summary>
    public Task InvokeAsync(HttpContext context)
    {
        var incoming = context.Request.Headers[HeaderName].FirstOrDefault();
        var id = IsAcceptable(incoming) ? incoming! : Guid.NewGuid().ToString("N");
        context.Items[HeaderName] = id;

        // Headers must be set before the first byte is written, so hook into "response starting".
        context.Response.OnStarting(() =>
        {
            context.Response.Headers[HeaderName] = id;
            return Task.CompletedTask;
        });

        return next(context);
    }

    static bool IsAcceptable(string? value) =>
        value is { Length: >= 1 and <= 64 } && value.All(c => char.IsAsciiLetterOrDigit(c) || c is '.' or '_' or '-');
}

public static class RequestIdExtensions
{
    public static IApplicationBuilder UseRequestId(this IApplicationBuilder app) =>
        app.UseMiddleware<RequestIdMiddleware>();
}
