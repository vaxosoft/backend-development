using Microsoft.Extensions.Options;
using P4.ApiAggregator;

var builder = WebApplication.CreateBuilder(args);

// Bound lazily (not read here) so tests can override configuration before the host is built.
builder.Services.AddOptions<AggregatorOptions>().BindConfiguration(AggregatorOptions.SectionName);
builder.Services.AddHttpClient<UpstreamClient>((sp, client) =>
{
    var baseUrl = sp.GetRequiredService<IOptions<AggregatorOptions>>().Value.UpstreamBaseUrl;
    client.BaseAddress = new Uri(baseUrl.EndsWith('/') ? baseUrl : baseUrl + "/");
    client.Timeout = Timeout.InfiniteTimeSpan; // per-attempt timeouts are enforced by UpstreamClient
});
builder.Services.AddTransient<Aggregator>();

var app = builder.Build();

app.MapGet("/aggregate", async (string? ids, Aggregator aggregator, CancellationToken cancellationToken) =>
{
    if (!IdParser.TryParse(ids, out var parsed, out var error))
    {
        return Results.Problem(detail: error, statusCode: StatusCodes.Status400BadRequest, title: "Bad Request");
    }

    return Results.Ok(await aggregator.AggregateAsync(parsed, cancellationToken));
});

// ADDR (host:port) controls the listening address. Ctrl-C / SIGTERM stop the host gracefully.
app.Run($"http://{Environment.GetEnvironmentVariable("ADDR") ?? "127.0.0.1:8080"}");

public partial class Program;
