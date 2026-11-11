using System.Collections.Concurrent;

namespace P7.TestedService;

public sealed record TaskItem(int Id, string Title, string Status, DateTimeOffset CreatedAt);

public sealed record CreateTaskRequest(string? Title);

/// <summary>The seam tests replace: the endpoints only know this interface.</summary>
public interface ITaskStore
{
    TaskItem Add(string title);

    TaskItem? Get(int id);

    IReadOnlyList<TaskItem> List();
}

public sealed class InMemoryTaskStore(TimeProvider time) : ITaskStore
{
    private readonly ConcurrentDictionary<int, TaskItem> _tasks = new();
    private int _lastId;

    public TaskItem Add(string title)
    {
        var task = new TaskItem(Interlocked.Increment(ref _lastId), title, "todo", time.GetUtcNow());
        _tasks[task.Id] = task;
        return task;
    }

    public TaskItem? Get(int id) => _tasks.GetValueOrDefault(id);

    public IReadOnlyList<TaskItem> List() => [.. _tasks.Values.OrderBy(t => t.Id)];
}

public static class TaskEndpoints
{
    public static IEndpointRouteBuilder MapTasks(this IEndpointRouteBuilder app)
    {
        var tasks = app.MapGroup("/tasks");

        tasks.MapGet("/", (ITaskStore store) =>
        {
            var items = store.List();
            return Results.Ok(new { items, total = items.Count });
        });

        tasks.MapGet("/{id:int}", (int id, ITaskStore store) =>
            store.Get(id) is { } task ? Results.Ok(task) : Results.Problem("No such task.", statusCode: 404));

        tasks.MapPost("/", (CreateTaskRequest request, ITaskStore store) =>
        {
            var title = request.Title?.Trim() ?? string.Empty;
            if (title.Length is < 1 or > 200)
            {
                return Results.ValidationProblem(
                    new Dictionary<string, string[]> { ["title"] = ["must be 1-200 characters"] },
                    statusCode: 422);
            }

            var task = store.Add(title);
            return Results.Created($"/tasks/{task.Id}", task);
        });

        return app;
    }
}
