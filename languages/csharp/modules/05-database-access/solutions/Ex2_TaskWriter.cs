using Microsoft.Data.Sqlite;

namespace M05.Exercises;

public static class Ex2_TaskWriter
{
    /// <summary>
    /// Insert the task and all its tags atomically and return the new task id.
    /// If anything fails (duplicate tag, constraint), nothing may remain in the database and the exception propagates.
    /// </summary>
    public static async Task<long> CreateWithTagsAsync(SqliteConnection conn, string title, IReadOnlyList<string> tags, CancellationToken ct = default)
    {
        await using var tx = (SqliteTransaction)await conn.BeginTransactionAsync(ct);

        await using var insert = conn.CreateCommand();
        insert.Transaction = tx;
        insert.CommandText = "INSERT INTO tasks (title) VALUES (@title) RETURNING id";
        insert.Parameters.AddWithValue("@title", title);
        var id = (long)(await insert.ExecuteScalarAsync(ct))!;

        await InsertTagsAsync(conn, tx, id, tags, ct);
        await tx.CommitAsync(ct);
        return id;
    }

    /// <summary>
    /// Replace all tags of a task (delete, then insert) atomically. If a new tag fails to insert,
    /// the old tags must still be there afterwards and the exception propagates.
    /// </summary>
    public static async Task ReplaceTagsAsync(SqliteConnection conn, long taskId, IReadOnlyList<string> tags, CancellationToken ct = default)
    {
        await using var tx = (SqliteTransaction)await conn.BeginTransactionAsync(ct);

        await using (var delete = conn.CreateCommand())
        {
            delete.Transaction = tx;
            delete.CommandText = "DELETE FROM task_tags WHERE task_id = @id";
            delete.Parameters.AddWithValue("@id", taskId);
            await delete.ExecuteNonQueryAsync(ct);
        }

        await InsertTagsAsync(conn, tx, taskId, tags, ct);
        await tx.CommitAsync(ct);
    }

    // Disposing an uncommitted transaction rolls it back, so a thrown exception leaves nothing behind.
    private static async Task InsertTagsAsync(SqliteConnection conn, SqliteTransaction tx, long taskId, IReadOnlyList<string> tags, CancellationToken ct)
    {
        foreach (var tag in tags)
        {
            await using var cmd = conn.CreateCommand();
            cmd.Transaction = tx;
            cmd.CommandText = "INSERT INTO task_tags (task_id, tag) VALUES (@id, @tag)";
            cmd.Parameters.AddWithValue("@id", taskId);
            cmd.Parameters.AddWithValue("@tag", tag);
            await cmd.ExecuteNonQueryAsync(ct);
        }
    }
}
