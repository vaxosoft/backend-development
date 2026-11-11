namespace P2.TasksCrudApi;

public static class TaskEndpoints
{
    public const int DefaultLimit = 20;
    public const int MaxLimit = 100;

    private static readonly string[] Sorts = ["created_at", "-created_at", "due_date", "-due_date"];

    public static void MapTaskEndpoints(this IEndpointRouteBuilder app)
    {
        var tasks = app.MapGroup("/tasks");
        tasks.MapPost("", CreateAsync);
        tasks.MapGet("", ListAsync);
        tasks.MapGet("/{id:long}", GetAsync);
        tasks.MapPut("/{id:long}", ReplaceAsync);
        tasks.MapPatch("/{id:long}", PatchAsync);
        tasks.MapDelete("/{id:long}", DeleteAsync);
    }

    private static async Task<IResult> CreateAsync(TaskRequest request, ITaskRepository repo, CancellationToken ct)
    {
        var (data, errors) = TaskValidator.Validate(request);
        if (data is null)
        {
            return Invalid(errors);
        }

        var task = await repo.CreateAsync(data, ct);
        return Results.Created($"/tasks/{task.Id}", task);
    }

    private static async Task<IResult> ListAsync(
        string? status, string? tag, int? limit, int? offset, string? sort, ITaskRepository repo, CancellationToken ct)
    {
        if (status is not null && !TaskValidator.Statuses.Contains(status))
        {
            return BadRequest("status must be one of todo, in_progress, done");
        }

        if (limit is < 1)
        {
            return BadRequest("limit must be at least 1");
        }

        if (offset is < 0)
        {
            return BadRequest("offset must not be negative");
        }

        if (sort is not null && !Sorts.Contains(sort))
        {
            return BadRequest($"sort must be one of {string.Join(", ", Sorts)}");
        }

        var query = new TaskQuery(status, tag, Math.Min(limit ?? DefaultLimit, MaxLimit), offset ?? 0, sort ?? "-created_at");
        return Results.Ok(await repo.ListAsync(query, ct));
    }

    private static async Task<IResult> GetAsync(long id, ITaskRepository repo, CancellationToken ct) =>
        await repo.GetAsync(id, ct) is { } task ? Results.Ok(task) : NotFound();

    private static async Task<IResult> ReplaceAsync(long id, TaskRequest request, ITaskRepository repo, CancellationToken ct)
    {
        var (data, errors) = TaskValidator.Validate(request);
        if (data is null)
        {
            return Invalid(errors);
        }

        return await repo.ReplaceAsync(id, data, ct) is { } task ? Results.Ok(task) : NotFound();
    }

    private static async Task<IResult> PatchAsync(long id, TaskPatch patch, ITaskRepository repo, CancellationToken ct)
    {
        if (await repo.GetAsync(id, ct) is not { } existing)
        {
            return NotFound();
        }

        var (data, errors) = TaskValidator.Apply(existing, patch);
        if (data is null)
        {
            return Invalid(errors);
        }

        return await repo.ReplaceAsync(id, data, ct) is { } task ? Results.Ok(task) : NotFound();
    }

    private static async Task<IResult> DeleteAsync(long id, ITaskRepository repo, CancellationToken ct) =>
        await repo.DeleteAsync(id, ct) ? Results.NoContent() : NotFound();

    private static IResult NotFound() => Results.Problem(statusCode: StatusCodes.Status404NotFound, title: "Task not found");

    private static IResult BadRequest(string detail) =>
        Results.Problem(statusCode: StatusCodes.Status400BadRequest, title: "Bad request", detail: detail);

    private static IResult Invalid(Dictionary<string, string[]> errors) =>
        Results.ValidationProblem(errors, statusCode: StatusCodes.Status422UnprocessableEntity, title: "Invalid task");
}
