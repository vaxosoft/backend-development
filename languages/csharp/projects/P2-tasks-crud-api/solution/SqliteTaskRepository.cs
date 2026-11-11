using System.Globalization;
using System.Text.Json;
using Microsoft.Data.Sqlite;

namespace P2.TasksCrudApi;

/// <summary>SQLite implementation (ADO.NET). Tags are stored as a JSON array in one column.</summary>
public sealed class SqliteTaskRepository : ITaskRepository, IDisposable
{
    private const string Columns = "id, title, status, due_date, tags, owner_id, created_at";
    private const string TimestampFormat = "yyyy-MM-dd'T'HH:mm:ss'Z'";

    private readonly string _connectionString;
    private readonly TimeProvider _time;
    private readonly SqliteConnection _keepAlive;

    public SqliteTaskRepository(string connectionString, TimeProvider time)
    {
        _connectionString = connectionString;
        _time = time;

        // A shared in-memory database lives only while some connection is open, so one stays open.
        _keepAlive = Open();
        using var create = _keepAlive.CreateCommand();
        create.CommandText = """
            CREATE TABLE IF NOT EXISTS tasks (
                id         INTEGER PRIMARY KEY AUTOINCREMENT,
                title      TEXT NOT NULL,
                status     TEXT NOT NULL,
                due_date   TEXT NULL,
                tags       TEXT NOT NULL,
                owner_id   INTEGER NULL,
                created_at TEXT NOT NULL
            )
            """;
        create.ExecuteNonQuery();
    }

    public void Dispose() => _keepAlive.Dispose();

    public async Task<TaskItem> CreateAsync(TaskData data, CancellationToken ct)
    {
        await using var connection = await OpenAsync(ct);
        await using var command = connection.CreateCommand();
        command.CommandText = $"""
            INSERT INTO tasks (title, status, due_date, tags, owner_id, created_at)
            VALUES (@title, @status, @due_date, @tags, @owner_id, @created_at)
            RETURNING {Columns}
            """;
        BindData(command, data);
        command.Parameters.AddWithValue("@created_at", Now());

        await using var row = await command.ExecuteReaderAsync(ct);
        await row.ReadAsync(ct);
        return Read(row);
    }

    public async Task<TaskItem?> GetAsync(long id, CancellationToken ct)
    {
        await using var connection = await OpenAsync(ct);
        await using var command = connection.CreateCommand();
        command.CommandText = $"SELECT {Columns} FROM tasks WHERE id = @id";
        command.Parameters.AddWithValue("@id", id);

        await using var row = await command.ExecuteReaderAsync(ct);
        return await row.ReadAsync(ct) ? Read(row) : null;
    }

    public async Task<TaskPage> ListAsync(TaskQuery query, CancellationToken ct)
    {
        // ORDER BY cannot be parameterised, so it comes from a fixed whitelist, never from user text.
        var orderBy = query.Sort switch
        {
            "created_at" => "created_at ASC, id ASC",
            "-created_at" => "created_at DESC, id DESC",
            "due_date" => "due_date IS NULL, due_date ASC, id ASC",
            "-due_date" => "due_date IS NULL, due_date DESC, id ASC",
            _ => throw new ArgumentException($"unsupported sort '{query.Sort}'", nameof(query)),
        };
        const string where = """
            WHERE (@status IS NULL OR status = @status)
              AND (@tag IS NULL OR EXISTS (SELECT 1 FROM json_each(tasks.tags) WHERE value = @tag))
            """;

        await using var connection = await OpenAsync(ct);

        await using var count = connection.CreateCommand();
        count.CommandText = $"SELECT COUNT(*) FROM tasks {where}";
        BindFilter(count, query);
        var total = Convert.ToInt32(await count.ExecuteScalarAsync(ct), CultureInfo.InvariantCulture);

        await using var select = connection.CreateCommand();
        select.CommandText = $"SELECT {Columns} FROM tasks {where} ORDER BY {orderBy} LIMIT @limit OFFSET @offset";
        BindFilter(select, query);
        select.Parameters.AddWithValue("@limit", query.Limit);
        select.Parameters.AddWithValue("@offset", query.Offset);

        var items = new List<TaskItem>();
        await using var rows = await select.ExecuteReaderAsync(ct);
        while (await rows.ReadAsync(ct))
        {
            items.Add(Read(rows));
        }

        return new TaskPage(items, total, query.Limit, query.Offset);
    }

    public async Task<TaskItem?> ReplaceAsync(long id, TaskData data, CancellationToken ct)
    {
        await using var connection = await OpenAsync(ct);
        await using var command = connection.CreateCommand();
        command.CommandText = $"""
            UPDATE tasks
            SET title = @title, status = @status, due_date = @due_date, tags = @tags, owner_id = @owner_id
            WHERE id = @id
            RETURNING {Columns}
            """;
        BindData(command, data);
        command.Parameters.AddWithValue("@id", id);

        await using var row = await command.ExecuteReaderAsync(ct);
        return await row.ReadAsync(ct) ? Read(row) : null;
    }

    public async Task<bool> DeleteAsync(long id, CancellationToken ct)
    {
        await using var connection = await OpenAsync(ct);
        await using var command = connection.CreateCommand();
        command.CommandText = "DELETE FROM tasks WHERE id = @id";
        command.Parameters.AddWithValue("@id", id);
        return await command.ExecuteNonQueryAsync(ct) > 0;
    }

    private SqliteConnection Open()
    {
        var connection = new SqliteConnection(_connectionString);
        connection.Open();
        return connection;
    }

    private async Task<SqliteConnection> OpenAsync(CancellationToken ct)
    {
        var connection = new SqliteConnection(_connectionString);
        await connection.OpenAsync(ct);
        return connection;
    }

    private string Now() => _time.GetUtcNow().UtcDateTime.ToString(TimestampFormat, CultureInfo.InvariantCulture);

    private static void BindData(SqliteCommand command, TaskData data)
    {
        command.Parameters.AddWithValue("@title", data.Title);
        command.Parameters.AddWithValue("@status", data.Status);
        command.Parameters.AddWithValue("@due_date", data.DueDate?.ToString("yyyy-MM-dd", CultureInfo.InvariantCulture) ?? (object)DBNull.Value);
        command.Parameters.AddWithValue("@tags", JsonSerializer.Serialize(data.Tags));
        command.Parameters.AddWithValue("@owner_id", data.OwnerId ?? (object)DBNull.Value);
    }

    private static void BindFilter(SqliteCommand command, TaskQuery query)
    {
        command.Parameters.AddWithValue("@status", query.Status ?? (object)DBNull.Value);
        command.Parameters.AddWithValue("@tag", query.Tag ?? (object)DBNull.Value);
    }

    /// <summary>Maps a row selected with <see cref="Columns"/>.</summary>
    private static TaskItem Read(SqliteDataReader row) => new(
        row.GetInt64(0),
        row.GetString(1),
        row.GetString(2),
        row.IsDBNull(3) ? null : DateOnly.ParseExact(row.GetString(3), "yyyy-MM-dd", CultureInfo.InvariantCulture),
        JsonSerializer.Deserialize<List<string>>(row.GetString(4)) ?? [],
        row.IsDBNull(5) ? null : row.GetInt64(5),
        DateTimeOffset.Parse(row.GetString(6), CultureInfo.InvariantCulture, DateTimeStyles.AssumeUniversal));
}
