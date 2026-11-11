namespace P1.HelloHttp;

public static class Endpoints
{
    public const int MaxSlowMs = 2000;

    public static void MapEndpoints(this IEndpointRouteBuilder app)
    {
        app.MapGet("/healthz", () => Results.Ok(new { status = "ok" }));

        app.MapGet("/hello/{name?}", (string? name) => Results.Text($"Hello, {name ?? "world"}!"));

        app.MapGet("/slow", async (string? ms, TimeProvider time, CancellationToken ct) =>
        {
            if (!int.TryParse(ms ?? "0", out var millis) || millis < 0)
            {
                return Results.Problem(statusCode: StatusCodes.Status400BadRequest, title: "ms must be a non-negative integer");
            }

            await Task.Delay(TimeSpan.FromMilliseconds(Math.Min(millis, MaxSlowMs)), time, ct);
            return Results.Text("done");
        });

        app.MapGet("/panic", Panic);
    }

    private static IResult Panic() => throw new InvalidOperationException("simulated failure");
}
