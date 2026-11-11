namespace P2.TasksCrudApi;

/// <summary>Storage abstraction: the endpoints know nothing about SQL.</summary>
public interface ITaskRepository
{
    Task<TaskItem> CreateAsync(TaskData data, CancellationToken ct);

    Task<TaskItem?> GetAsync(long id, CancellationToken ct);

    Task<TaskPage> ListAsync(TaskQuery query, CancellationToken ct);

    /// <returns>The updated task, or <c>null</c> when the id does not exist. <c>CreatedAt</c> never changes.</returns>
    Task<TaskItem?> ReplaceAsync(long id, TaskData data, CancellationToken ct);

    /// <returns><c>true</c> when a row was deleted.</returns>
    Task<bool> DeleteAsync(long id, CancellationToken ct);
}
