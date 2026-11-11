using System.Globalization;
using Microsoft.Data.Sqlite;

namespace M05.Examples;

/// <summary>NULL in both directions and mapping a row to a record.</summary>
public static class NullsAndMapping
{
    private sealed record TaskRow(int Id, string Title, DateOnly? DueDate, int? OwnerId);

    public static async Task<int> RunAsync(string[] args)
    {
        await using var conn = new SqliteConnection("Data Source=:memory:");
        await conn.OpenAsync();
        await Exec(conn, "CREATE TABLE tasks (id INTEGER PRIMARY KEY, title TEXT NOT NULL, due_date TEXT, owner_id INTEGER)");

        await Insert(conn, "With everything", new DateOnly(2030, 1, 31), 7);
        await Insert(conn, "With nothing", null, null);

        // A C# null is not a database NULL: it must become DBNull.Value.
        await using (var cmd = conn.CreateCommand())
        {
            cmd.CommandText = "INSERT INTO tasks (title, due_date) VALUES ('x', @due)";
            cmd.Parameters.AddWithValue("@due", null);
            try
            {
                await cmd.ExecuteNonQueryAsync();
            }
            catch (InvalidOperationException e)
            {
                Console.WriteLine($"null parameter: {e.GetType().Name}: {e.Message}");
            }
        }

        await using var select = conn.CreateCommand();
        select.CommandText = "SELECT id, title, due_date, owner_id FROM tasks ORDER BY id";
        await using var reader = await select.ExecuteReaderAsync();

        var rows = new List<TaskRow>();
        while (await reader.ReadAsync())
        {
            if (rows.Count == 1)
            {
                try
                {
                    reader.GetInt32(3);
                }
                catch (InvalidOperationException e)
                {
                    Console.WriteLine($"GetInt32 on NULL: {e.Message.Split('.')[0]}");
                }
            }

            rows.Add(Map(reader));
        }

        foreach (var row in rows)
        {
            Console.WriteLine($"{row.Id} {row.Title} due={row.DueDate?.ToString("O") ?? "null"} owner={row.OwnerId?.ToString() ?? "null"}");
        }

        return 0;
    }

    private static TaskRow Map(SqliteDataReader r) => new(
        r.GetInt32(r.GetOrdinal("id")),
        r.GetString(r.GetOrdinal("title")),
        r.IsDBNull(r.GetOrdinal("due_date")) ? null : DateOnly.ParseExact(r.GetString(r.GetOrdinal("due_date")), "yyyy-MM-dd", CultureInfo.InvariantCulture),
        r.IsDBNull(r.GetOrdinal("owner_id")) ? null : r.GetInt32(r.GetOrdinal("owner_id")));

    private static async Task Insert(SqliteConnection conn, string title, DateOnly? due, int? owner)
    {
        await using var cmd = conn.CreateCommand();
        cmd.CommandText = "INSERT INTO tasks (title, due_date, owner_id) VALUES (@title, @due, @owner)";
        cmd.Parameters.AddWithValue("@title", title);
        cmd.Parameters.AddWithValue("@due", due?.ToString("yyyy-MM-dd", CultureInfo.InvariantCulture) ?? (object)DBNull.Value);
        cmd.Parameters.AddWithValue("@owner", owner ?? (object)DBNull.Value);
        await cmd.ExecuteNonQueryAsync();
    }

    private static async Task Exec(SqliteConnection conn, string sql)
    {
        await using var cmd = conn.CreateCommand();
        cmd.CommandText = sql;
        await cmd.ExecuteNonQueryAsync();
    }
}
