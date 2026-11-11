using Microsoft.Data.Sqlite;
using M12.TaskApi;

namespace M12.Examples;

public static class SqliteIsolation
{
    private static long Count(SqliteConnection db)
    {
        using var command = db.CreateCommand();
        command.CommandText = "SELECT COUNT(*) FROM t";
        return (long)command.ExecuteScalar()!;
    }

    private static void Run(SqliteConnection db, string sql)
    {
        using var command = db.CreateCommand();
        command.CommandText = sql;
        command.ExecuteNonQuery();
    }

    public static async Task<int> RunAsync(string[] args)
    {
        using (var db = new SqliteConnection("Data Source=:memory:"))
        {
            db.Open();
            Run(db, "CREATE TABLE t (x INT); INSERT INTO t VALUES (1)");
            Console.WriteLine($"same connection sees the table: {Count(db)} row");

            db.Close();
            db.Open();
            try
            {
                Count(db);
            }
            catch (SqliteException ex)
            {
                Console.WriteLine($"after Close()+Open() the database is gone: {ex.Message}");
            }
        }

        // One fresh store per test = a fresh database; nothing leaks between tests.
        using var first = new SqliteTaskStore();
        using var second = new SqliteTaskStore();
        await first.AddAsync(new NewTask("only in first", null), DateTimeOffset.UnixEpoch);
        Console.WriteLine($"first store has {(await first.ListAsync()).Count} task, second has {(await second.ListAsync()).Count}");
        return 0;
    }
}
