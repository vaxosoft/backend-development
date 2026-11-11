using System.Globalization;
using Microsoft.Data.Sqlite;

namespace M12.TaskApi;

/// <summary>
/// SQLite-backed store. An in-memory database lives only as long as its connection, so this
/// object owns one open connection for its whole lifetime and serializes access to it.
/// </summary>
public sealed class SqliteTaskStore : ITaskStore, IDisposable
{
    private const string Columns = "id, title, status, due_date, created_at";

    private readonly SqliteConnection _db = new("Data Source=:memory:");
    private readonly Lock _gate = new();

    public SqliteTaskStore()
    {
        _db.Open();
        Execute("""
            CREATE TABLE tasks (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                title TEXT NOT NULL,
                status TEXT NOT NULL DEFAULT 'todo',
                due_date TEXT NULL,
                created_at TEXT NOT NULL)
            """);
    }

    public Task<TaskItem> AddAsync(NewTask task, DateTimeOffset createdAt, CancellationToken ct = default)
    {
        lock (_gate)
        {
            using var command = _db.CreateCommand();
            command.CommandText = $"INSERT INTO tasks (title, due_date, created_at) VALUES (@title, @due, @created) RETURNING {Columns}";
            command.Parameters.AddWithValue("@title", task.Title);
            command.Parameters.AddWithValue("@due", task.DueDate?.ToString("yyyy-MM-dd", CultureInfo.InvariantCulture) ?? (object)DBNull.Value);
            command.Parameters.AddWithValue("@created", createdAt.ToString("O", CultureInfo.InvariantCulture));
            return Task.FromResult(ReadAll(command).Single());
        }
    }

    public Task<TaskItem?> GetAsync(int id, CancellationToken ct = default)
    {
        lock (_gate)
        {
            using var command = _db.CreateCommand();
            command.CommandText = $"SELECT {Columns} FROM tasks WHERE id = @id";
            command.Parameters.AddWithValue("@id", id);
            return Task.FromResult(ReadAll(command).SingleOrDefault());
        }
    }

    public Task<IReadOnlyList<TaskItem>> ListAsync(string? status = null, CancellationToken ct = default)
    {
        lock (_gate)
        {
            using var command = _db.CreateCommand();
            command.CommandText = $"SELECT {Columns} FROM tasks WHERE @status IS NULL OR status = @status ORDER BY id";
            command.Parameters.AddWithValue("@status", status ?? (object)DBNull.Value);
            return Task.FromResult<IReadOnlyList<TaskItem>>(ReadAll(command));
        }
    }

    public Task<TaskItem?> SetStatusAsync(int id, string status, CancellationToken ct = default)
    {
        lock (_gate)
        {
            using var command = _db.CreateCommand();
            command.CommandText = $"UPDATE tasks SET status = @status WHERE id = @id RETURNING {Columns}";
            command.Parameters.AddWithValue("@status", status);
            command.Parameters.AddWithValue("@id", id);
            return Task.FromResult(ReadAll(command).SingleOrDefault());
        }
    }

    public Task<bool> DeleteAsync(int id, CancellationToken ct = default)
    {
        lock (_gate)
        {
            using var command = _db.CreateCommand();
            command.CommandText = "DELETE FROM tasks WHERE id = @id";
            command.Parameters.AddWithValue("@id", id);
            return Task.FromResult(command.ExecuteNonQuery() > 0);
        }
    }

    public void Dispose() => _db.Dispose();

    private void Execute(string sql)
    {
        using var command = _db.CreateCommand();
        command.CommandText = sql;
        command.ExecuteNonQuery();
    }

    private static List<TaskItem> ReadAll(SqliteCommand command)
    {
        using var reader = command.ExecuteReader();
        var items = new List<TaskItem>();
        while (reader.Read())
        {
            items.Add(new TaskItem(
                reader.GetInt32(0),
                reader.GetString(1),
                reader.GetString(2),
                reader.IsDBNull(3) ? null : DateOnly.ParseExact(reader.GetString(3), "yyyy-MM-dd", CultureInfo.InvariantCulture),
                DateTimeOffset.Parse(reader.GetString(4), CultureInfo.InvariantCulture)));
        }

        return items;
    }
}
