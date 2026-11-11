namespace P2.TasksCrudApi;

public static class TaskEndpoints
{
    public const int DefaultLimit = 20;
    public const int MaxLimit = 100;

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

    private static Task<IResult> CreateAsync(TaskRequest request, ITaskRepository repo, CancellationToken ct) =>
        throw new NotImplementedException("TODO: validate (422), store, 201 + Location: /tasks/{id}");

    private static Task<IResult> ListAsync(
        string? status, string? tag, int? limit, int? offset, string? sort, ITaskRepository repo, CancellationToken ct) =>
        throw new NotImplementedException("TODO: validate the query (400), clamp limit, return {items,total,limit,offset}");

    private static Task<IResult> GetAsync(long id, ITaskRepository repo, CancellationToken ct) =>
        throw new NotImplementedException("TODO: 200 or 404");

    private static Task<IResult> ReplaceAsync(long id, TaskRequest request, ITaskRepository repo, CancellationToken ct) =>
        throw new NotImplementedException("TODO: validate (422), replace, 200 or 404");

    private static Task<IResult> PatchAsync(long id, TaskPatch patch, ITaskRepository repo, CancellationToken ct) =>
        throw new NotImplementedException("TODO: load (404), apply + validate (422), replace, 200");

    private static Task<IResult> DeleteAsync(long id, ITaskRepository repo, CancellationToken ct) =>
        throw new NotImplementedException("TODO: 204 or 404");
}
