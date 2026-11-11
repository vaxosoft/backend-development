using Microsoft.Data.Sqlite;

namespace M05.Exercises;

public static class Ex2_TaskWriter
{
    /// <summary>
    /// Insert the task and all its tags atomically and return the new task id.
    /// If anything fails (duplicate tag, constraint), nothing may remain in the database and the exception propagates.
    /// </summary>
    public static Task<long> CreateWithTagsAsync(SqliteConnection conn, string title, IReadOnlyList<string> tags, CancellationToken ct = default) =>
        throw new NotImplementedException();

    /// <summary>
    /// Replace all tags of a task (delete, then insert) atomically. If a new tag fails to insert,
    /// the old tags must still be there afterwards and the exception propagates.
    /// </summary>
    public static Task ReplaceTagsAsync(SqliteConnection conn, long taskId, IReadOnlyList<string> tags, CancellationToken ct = default) =>
        throw new NotImplementedException();
}
