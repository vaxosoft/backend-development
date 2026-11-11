namespace P7.TestedService;

/// <summary>Flips to ready once startup work is done; orchestrators route traffic only to ready instances.</summary>
public sealed class ReadinessState
{
    private volatile bool _ready;

    public bool IsReady => _ready;

    public void MarkReady() => _ready = true;

    public void MarkNotReady() => _ready = false;
}

public sealed class RequestCounter
{
    private long _total;

    public long Total => Interlocked.Read(ref _total);

    public void Increment() => Interlocked.Increment(ref _total);
}

public static class OperationsEndpoints
{
    /// <summary>Counts every request that enters the pipeline, including the scrape of /metrics itself.</summary>
    public static IApplicationBuilder UseRequestCounting(this IApplicationBuilder app) =>
        app.Use(async (context, next) =>
        {
            context.RequestServices.GetRequiredService<RequestCounter>().Increment();
            await next(context);
        });

    public static IEndpointRouteBuilder MapOperations(this IEndpointRouteBuilder app)
    {
        app.MapGet("/healthz", (AppConfig config) => Results.Ok(new { status = "ok", env = config.Env }));

        app.MapGet("/readyz", (ReadinessState state) =>
            state.IsReady
                ? Results.Ok(new { status = "ready" })
                : Results.Json(new { status = "not_ready" }, statusCode: StatusCodes.Status503ServiceUnavailable));

        app.MapGet("/metrics", (RequestCounter counter) => Results.Text($"requests_total {counter.Total}\n"));

        return app;
    }
}
