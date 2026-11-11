using System.Text.Json;
using Microsoft.AspNetCore.Http.HttpResults;
using Microsoft.Extensions.Options;

namespace M03.Examples;

/// <summary>The complete Task Tracker REST API (in-memory): CRUD + PATCH + list, validation, problem details, OpenAPI.</summary>
static class TaskTrackerApi
{
    /// <summary>JSON merge-patch semantics: an absent member is unchanged, an explicit null clears the field.</summary>
    static Results<Ok<TaskItem>, ProblemHttpResult> Patch(int id, JsonElement body, TaskRepository repo)
    {
        if (body.ValueKind != JsonValueKind.Object)
        {
            return TypedResults.Problem(statusCode: StatusCodes.Status400BadRequest, title: "Bad Request", detail: "body must be a JSON object");
        }

        if (repo.Find(id) is not { } current)
        {
            return Problems.NotFound("task", id);
        }

        try
        {
            var merged = new TaskInput
            {
                Title = body.TryGetProperty("title", out var title) ? title.GetString() : current.Title,
                Status = body.TryGetProperty("status", out var status)
                    ? JsonSerializer.Deserialize<TaskState>(status, JsonOptions)
                    : current.Status,
                DueDate = body.TryGetProperty("due_date", out var due)
                    ? due.Deserialize<DateOnly?>(JsonOptions)
                    : current.DueDate,
                Tags = body.TryGetProperty("tags", out var tags)
                    ? tags.Deserialize<string[]>(JsonOptions) ?? []
                    : [.. current.Tags],
                OwnerId = current.OwnerId,
            };

            if (Problems.Validate(merged) is { } invalid)
            {
                return invalid;
            }

            return TypedResults.Ok(repo.Replace(id, merged)!);
        }
        catch (Exception ex) when (ex is JsonException or InvalidOperationException or FormatException)
        {
            return TypedResults.Problem(statusCode: StatusCodes.Status400BadRequest, title: "Bad Request", detail: "a field has the wrong type or format");
        }
    }

    static readonly JsonSerializerOptions JsonOptions = Configure();

    static JsonSerializerOptions Configure()
    {
        var options = new JsonSerializerOptions(JsonSerializerDefaults.Web);
        Json.Configure(options);
        return options;
    }

    public static async Task<int> RunAsync(string[] args)
    {
        var builder = ExampleHost.CreateBuilder(args);
        builder.Services.ConfigureHttpJsonOptions(o => Json.Configure(o.SerializerOptions));
        builder.Services.AddValidatedProblemDetails();
        builder.Services.AddOpenApi();
        builder.Services.AddSingleton<TimeProvider, StepClock>();
        builder.Services.AddSingleton<TaskRepository>();

        var app = builder.Build();
        app.UseStatusCodePages();
        app.MapOpenApi();

        var tasks = app.MapGroup("/tasks").WithTags("tasks");

        tasks.MapGet("/", Pagination.List).WithSummary("List tasks (filter, sort, paginate)");
        tasks.MapPost("/", (TaskInput input, TaskRepository repo) =>
            {
                var task = repo.Add(input);
                return TypedResults.Created($"/tasks/{task.Id}", task);
            })
            .WithSummary("Create a task");
        tasks.MapGet("/{id:int}", (int id, TaskRepository repo) =>
                repo.Find(id) is { } task ? (Results<Ok<TaskItem>, ProblemHttpResult>)TypedResults.Ok(task) : Problems.NotFound("task", id))
            .WithSummary("Get one task");
        tasks.MapPut("/{id:int}", (int id, TaskInput input, TaskRepository repo) =>
                repo.Replace(id, input) is { } task ? (Results<Ok<TaskItem>, ProblemHttpResult>)TypedResults.Ok(task) : Problems.NotFound("task", id))
            .WithSummary("Replace a task");
        tasks.MapPatch("/{id:int}", Patch).WithSummary("Change some fields of a task");
        tasks.MapDelete("/{id:int}", (int id, TaskRepository repo) =>
                repo.Remove(id) ? (Results<NoContent, ProblemHttpResult>)TypedResults.NoContent() : Problems.NotFound("task", id))
            .WithSummary("Delete a task");

        await app.RunAsync();
        return 0;
    }
}
