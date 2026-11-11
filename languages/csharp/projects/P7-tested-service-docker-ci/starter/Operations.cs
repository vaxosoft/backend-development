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
    public static IApplicationBuilder UseRequestCounting(this IApplicationBuilder app)
    {
        // TODO: add middleware that increments the RequestCounter (from context.RequestServices) and calls next.
        return app;
    }

    public static IEndpointRouteBuilder MapOperations(this IEndpointRouteBuilder app)
    {
        app.MapGet("/healthz", (AppConfig config) => Results.Ok(new { status = "ok", env = config.Env }));

        // TODO: GET /readyz -> 200 {"status":"ready"} when ReadinessState.IsReady, else 503 {"status":"not_ready"}.
        // TODO: GET /metrics -> text/plain "requests_total <n>\n" using RequestCounter.Total.
        return app;
    }
}
