namespace P1.HelloHttp;

public static class Endpoints
{
    public const int MaxSlowMs = 2000;

    public static void MapEndpoints(this IEndpointRouteBuilder app)
    {
        app.MapGet("/healthz", () => Results.Ok(new { status = "ok" }));

        // TODO: GET /hello and GET /hello/{name} -> text/plain "Hello, {name}!" ("world" without a name).
        app.MapGet("/hello/{name?}", (string? name) => Results.Text("TODO"));

        // TODO: GET /slow?ms=N -> wait N ms (clamped to MaxSlowMs) using the injected TimeProvider, then "done".
        app.MapGet("/slow", async (string? ms, TimeProvider time, CancellationToken ct) =>
        {
            await Task.CompletedTask;
            return Results.Text("TODO");
        });

        // TODO: GET /panic must throw so the exception middleware has something to handle.
        app.MapGet("/panic", () => Results.Ok());
    }
}
