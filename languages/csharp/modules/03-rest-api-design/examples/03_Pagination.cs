using Microsoft.AspNetCore.Http.HttpResults;
using Microsoft.AspNetCore.Mvc;

namespace M03.Examples;

/// <summary>Pagination, filtering and sorting of a collection resource with a bounded page size.</summary>
public static class Pagination
{
    public const int DefaultLimit = 20;
    public const int MaxLimit = 100;

    public sealed record Page<T>(IReadOnlyList<T> Items, int Total, int Limit, int Offset);

    /// <summary>Query string of GET /tasks. Everything is optional; strings are parsed here for precise errors.</summary>
    public sealed record ListQuery(
        int? Limit,
        int? Offset,
        string? Status,
        string? Tag,
        string? Sort);

    static readonly Dictionary<string, Func<IEnumerable<TaskItem>, IOrderedEnumerable<TaskItem>>> Sorts = new()
    {
        ["created_at"] = q => q.OrderBy(t => t.CreatedAt).ThenBy(t => t.Id),
        ["-created_at"] = q => q.OrderByDescending(t => t.CreatedAt).ThenByDescending(t => t.Id),
        ["due_date"] = q => q.OrderBy(t => t.DueDate is null).ThenBy(t => t.DueDate).ThenBy(t => t.Id),
        ["-due_date"] = q => q.OrderBy(t => t.DueDate is null).ThenByDescending(t => t.DueDate).ThenBy(t => t.Id),
    };

    public static Results<Ok<Page<TaskItem>>, ProblemHttpResult> List([AsParameters] ListQuery query, TaskRepository repo)
    {
        if (query.Limit is < 1)
        {
            return Bad("limit must be at least 1");
        }

        if (query.Offset is < 0)
        {
            return Bad("offset must not be negative");
        }

        var limit = Math.Min(query.Limit ?? DefaultLimit, MaxLimit); // clamp: a huge page size is not an error
        var offset = query.Offset ?? 0;

        if (!Sorts.TryGetValue(query.Sort ?? "-created_at", out var sort))
        {
            return Bad($"sort must be one of: {string.Join(", ", Sorts.Keys)}");
        }

        IEnumerable<TaskItem> items = repo.All();
        if (query.Status is not null)
        {
            if (!TaskStates.TryParse(query.Status, out var status))
            {
                return Bad("status must be todo, in_progress or done");
            }

            items = items.Where(t => t.Status == status);
        }

        if (query.Tag is not null)
        {
            items = items.Where(t => t.Tags.Contains(query.Tag));
        }

        var filtered = sort(items).ToList();
        return TypedResults.Ok(new Page<TaskItem>([.. filtered.Skip(offset).Take(limit)], filtered.Count, limit, offset));
    }

    static ProblemHttpResult Bad(string detail) =>
        TypedResults.Problem(statusCode: StatusCodes.Status400BadRequest, title: "Bad Request", detail: detail);

    public static async Task<int> RunAsync(string[] args)
    {
        var builder = ExampleHost.CreateBuilder(args);
        builder.Services.ConfigureHttpJsonOptions(o => Json.Configure(o.SerializerOptions));
        builder.Services.AddProblemDetails();
        builder.Services.AddSingleton<TimeProvider, StepClock>();
        builder.Services.AddSingleton<TaskRepository>();

        var app = builder.Build();
        app.UseStatusCodePages();

        var repo = app.Services.GetRequiredService<TaskRepository>();
        for (var i = 1; i <= 25; i++)
        {
            repo.Add(new TaskInput
            {
                Title = $"Task {i:00}",
                Status = i % 3 == 0 ? TaskState.Done : TaskState.Todo,
                DueDate = i % 5 == 0 ? new DateOnly(2030, 3, 31).AddDays(-i) : null,
                Tags = i % 2 == 0 ? ["even"] : ["odd"],
            });
        }

        app.MapGet("/tasks", List);

        await app.RunAsync();
        return 0;
    }
}
