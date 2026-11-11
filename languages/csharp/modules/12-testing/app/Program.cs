using System.Text.Json;
using M12.TaskApi;

var builder = WebApplication.CreateBuilder(args);
builder.Services.AddSingleton(TimeProvider.System);
builder.Services.AddSingleton<ITaskStore, SqliteTaskStore>();
builder.Services.ConfigureHttpJsonOptions(o =>
{
    o.SerializerOptions.PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower;
    o.SerializerOptions.Converters.Add(new UtcTimestampConverter());
});

var app = builder.Build();

app.MapPost("/tasks", async (NewTask body, ITaskStore store, TimeProvider time, CancellationToken ct) =>
{
    if (string.IsNullOrWhiteSpace(body.Title) || body.Title.Trim().Length > 200)
    {
        return Results.ValidationProblem(
            new Dictionary<string, string[]> { ["title"] = ["must be 1-200 characters"] },
            statusCode: StatusCodes.Status422UnprocessableEntity);
    }

    var created = await store.AddAsync(body with { Title = body.Title.Trim() }, time.GetUtcNow(), ct);
    return Results.Created($"/tasks/{created.Id}", created);
});

app.MapGet("/tasks/{id:int}", async (int id, ITaskStore store, CancellationToken ct) =>
    await store.GetAsync(id, ct) is { } task ? Results.Ok(task) : Results.NotFound());

app.MapGet("/tasks", async (string? status, ITaskStore store, CancellationToken ct) =>
{
    var items = await store.ListAsync(status, ct);
    return new { items, total = items.Count };
});

app.Run();

/// <summary>Exposes the entry point to <c>WebApplicationFactory&lt;Program&gt;</c> in test projects.</summary>
public partial class Program;
