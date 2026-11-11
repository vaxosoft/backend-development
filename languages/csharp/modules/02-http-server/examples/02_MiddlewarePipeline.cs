namespace M02.Examples;

/// <summary>The middleware pipeline: order matters, a middleware may short-circuit.</summary>
static class MiddlewarePipeline
{
    /// <summary>Convention-based middleware: echoes X-Request-ID or generates one.</summary>
    sealed class RequestIdMiddleware(RequestDelegate next)
    {
        public const string Header = "X-Request-ID";

        public async Task InvokeAsync(HttpContext context)
        {
            var id = context.Request.Headers[Header].FirstOrDefault() is { Length: > 0 and <= 64 } given
                ? given
                : Guid.NewGuid().ToString("N");
            context.Items[Header] = id;
            context.Response.OnStarting(() =>
            {
                context.Response.Headers[Header] = id;
                return Task.CompletedTask;
            });
            await next(context);
        }
    }

    static void Trace(HttpContext context, string step)
    {
        var trace = (List<string>)context.Items["trace"]!;
        trace.Add(step);
    }

    public static async Task<int> RunAsync(string[] args)
    {
        var builder = ExampleHost.CreateBuilder(args);
        var app = builder.Build();

        app.Use(async (context, next) =>
        {
            var trace = new List<string> { "outer-in" };
            context.Items["trace"] = trace;
            context.Response.OnStarting(() =>
            {
                context.Response.Headers["X-Pipeline"] = string.Join(" > ", trace);
                return Task.CompletedTask;
            });
            await next(context);
            trace.Add("outer-out"); // too late for the header: response already started
        });

        app.UseMiddleware<RequestIdMiddleware>();

        app.Use(async (context, next) =>
        {
            if (context.Request.Path.StartsWithSegments("/blocked"))
            {
                context.Response.StatusCode = StatusCodes.Status403Forbidden;
                await context.Response.WriteAsync("blocked by middleware");
                return; // short-circuit: next is never called
            }

            Trace(context, "inner-in");
            await next(context);
        });

        app.UseWhen(c => c.Request.Path.StartsWithSegments("/admin"), branch =>
            branch.Use((context, next) =>
            {
                Trace(context, "admin-only");
                return next(context);
            }));

        app.MapGet("/", (HttpContext context) =>
        {
            Trace(context, "endpoint");
            return $"request id: {context.Items[RequestIdMiddleware.Header]}";
        });
        app.MapGet("/admin/stats", (HttpContext context) =>
        {
            Trace(context, "endpoint");
            return "admin stats";
        });
        app.MapGet("/blocked", () => "never reached");

        await app.RunAsync();
        return 0;
    }
}
