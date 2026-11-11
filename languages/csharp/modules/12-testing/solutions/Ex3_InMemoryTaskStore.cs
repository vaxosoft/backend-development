using M12.TaskApi;

namespace M12.Exercises;

/// <summary>
/// A thread-safe in-memory <see cref="ITaskStore"/> that behaves like the SQLite one:
/// ids start at 1 and are never reused, new tasks have status "todo", lists are ordered by id,
/// and <c>Title</c> is stored as given (the HTTP layer trims it).
/// </summary>
public sealed class Ex3_InMemoryTaskStore : ITaskStore
{
    private readonly Lock _gate = new();
    private readonly SortedDictionary<int, TaskItem> _tasks = [];
    private int _lastId;

    public Task<TaskItem> AddAsync(NewTask task, DateTimeOffset createdAt, CancellationToken ct = default)
    {
        lock (_gate)
        {
            var item = new TaskItem(++_lastId, task.Title!, "todo", task.DueDate, createdAt);
            _tasks[item.Id] = item;
            return Task.FromResult(item);
        }
    }

    public Task<TaskItem?> GetAsync(int id, CancellationToken ct = default)
    {
        lock (_gate)
        {
            return Task.FromResult(_tasks.GetValueOrDefault(id));
        }
    }

    public Task<IReadOnlyList<TaskItem>> ListAsync(string? status = null, CancellationToken ct = default)
    {
        lock (_gate)
        {
            IReadOnlyList<TaskItem> items = _tasks.Values.Where(t => status is null || t.Status == status).ToList();
            return Task.FromResult(items);
        }
    }

    public Task<TaskItem?> SetStatusAsync(int id, string status, CancellationToken ct = default)
    {
        lock (_gate)
        {
            if (!_tasks.TryGetValue(id, out var existing))
            {
                return Task.FromResult<TaskItem?>(null);
            }

            var updated = existing with { Status = status };
            _tasks[id] = updated;
            return Task.FromResult<TaskItem?>(updated);
        }
    }

    public Task<bool> DeleteAsync(int id, CancellationToken ct = default)
    {
        lock (_gate)
        {
            return Task.FromResult(_tasks.Remove(id));
        }
    }
}
