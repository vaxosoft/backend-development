using M12.TaskApi;

namespace M12.Exercises;

/// <summary>
/// A thread-safe in-memory <see cref="ITaskStore"/> that behaves like the SQLite one:
/// ids start at 1 and are never reused, new tasks have status "todo", lists are ordered by id,
/// and <c>Title</c> is stored as given (the HTTP layer trims it).
/// </summary>
public sealed class Ex3_InMemoryTaskStore : ITaskStore
{
    public Task<TaskItem> AddAsync(NewTask task, DateTimeOffset createdAt, CancellationToken ct = default) =>
        throw new NotImplementedException();

    public Task<TaskItem?> GetAsync(int id, CancellationToken ct = default) =>
        throw new NotImplementedException();

    public Task<IReadOnlyList<TaskItem>> ListAsync(string? status = null, CancellationToken ct = default) =>
        throw new NotImplementedException();

    public Task<TaskItem?> SetStatusAsync(int id, string status, CancellationToken ct = default) =>
        throw new NotImplementedException();

    public Task<bool> DeleteAsync(int id, CancellationToken ct = default) =>
        throw new NotImplementedException();
}
