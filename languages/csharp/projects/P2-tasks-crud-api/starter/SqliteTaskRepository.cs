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

    public Task<TaskItem> CreateAsync(TaskData data, CancellationToken ct) =>
        throw new NotImplementedException("TODO: INSERT the task (created_at from the TimeProvider) and return it");

    public Task<TaskItem?> GetAsync(long id, CancellationToken ct) =>
        throw new NotImplementedException("TODO: SELECT by id");

    public Task<TaskPage> ListAsync(TaskQuery query, CancellationToken ct) =>
        throw new NotImplementedException("TODO: filter, whitelisted ORDER BY, LIMIT/OFFSET and the total count");

    public Task<TaskItem?> ReplaceAsync(long id, TaskData data, CancellationToken ct) =>
        throw new NotImplementedException("TODO: UPDATE everything except id and created_at");

    public Task<bool> DeleteAsync(long id, CancellationToken ct) =>
        throw new NotImplementedException("TODO: DELETE by id");

    private SqliteConnection Open()
    {
        var connection = new SqliteConnection(_connectionString);
        connection.Open();
        return connection;
    }

    private string Now() => _time.GetUtcNow().UtcDateTime.ToString(TimestampFormat, CultureInfo.InvariantCulture);

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
