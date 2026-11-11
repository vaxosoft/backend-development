using System.Data;
using Microsoft.Data.Sqlite;

namespace M05.Examples;

/// <summary>Commit, rollback, and why "using var tx" is a safety net.</summary>
public static class Transactions
{
    public static async Task<int> RunAsync(string[] args)
    {
        await using var conn = new SqliteConnection("Data Source=:memory:");
        await conn.OpenAsync();
        await Exec(conn, "CREATE TABLE tasks (id INTEGER PRIMARY KEY, title TEXT NOT NULL)");
        await Exec(conn, "CREATE TABLE task_tags (task_id INTEGER NOT NULL REFERENCES tasks(id), tag TEXT NOT NULL, PRIMARY KEY (task_id, tag))");

        Console.WriteLine("commit : " + await CreateWithTags(conn, "Write docs", ["docs", "p1"]));
        Console.WriteLine("failure: " + await CreateWithTags(conn, "Broken", ["a", "a"]));
        Console.WriteLine($"tasks={await Count(conn, "tasks")} tags={await Count(conn, "task_tags")}");

        // Explicit rollback, and an isolation level (SQLite supports Serializable, the default, and ReadUncommitted).
        await using (var tx = (SqliteTransaction)await conn.BeginTransactionAsync(IsolationLevel.Serializable))
        {
            await Exec(conn, "DELETE FROM task_tags", tx);
            Console.WriteLine($"inside tx : tags={await Count(conn, "task_tags", tx)}");
            await tx.RollbackAsync();
        }

        Console.WriteLine($"after undo: tags={await Count(conn, "task_tags")}");
        return 0;
    }

    private static async Task<string> CreateWithTags(SqliteConnection conn, string title, string[] tags)
    {
        await using var tx = (SqliteTransaction)await conn.BeginTransactionAsync();
        try
        {
            await using var insert = conn.CreateCommand();
            insert.Transaction = tx;
            insert.CommandText = "INSERT INTO tasks (title) VALUES (@title) RETURNING id";
            insert.Parameters.AddWithValue("@title", title);
            var id = (long)(await insert.ExecuteScalarAsync())!;

            foreach (var tag in tags)
            {
                await using var tagCmd = conn.CreateCommand();
                tagCmd.Transaction = tx;
                tagCmd.CommandText = "INSERT INTO task_tags (task_id, tag) VALUES (@id, @tag)";
                tagCmd.Parameters.AddWithValue("@id", id);
                tagCmd.Parameters.AddWithValue("@tag", tag);
                await tagCmd.ExecuteNonQueryAsync();
            }

            await tx.CommitAsync();
            return $"committed task {id}";
        }
        catch (SqliteException e)
        {
            await tx.RollbackAsync();
            return $"rolled back (SQLite error {e.SqliteErrorCode}: constraint failed)";
        }
    }

    private static async Task<long> Count(SqliteConnection conn, string table, SqliteTransaction? tx = null)
    {
        await using var cmd = conn.CreateCommand();
        cmd.Transaction = tx;
        cmd.CommandText = $"SELECT count(*) FROM {table}";
        return (long)(await cmd.ExecuteScalarAsync())!;
    }

    private static async Task Exec(SqliteConnection conn, string sql, SqliteTransaction? tx = null)
    {
        await using var cmd = conn.CreateCommand();
        cmd.Transaction = tx;
        cmd.CommandText = sql;
        await cmd.ExecuteNonQueryAsync();
    }
}
