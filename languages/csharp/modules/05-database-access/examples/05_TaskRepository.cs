using System.Globalization;
using System.Text.Json;
using Microsoft.Data.Sqlite;

namespace M05.Examples;

public sealed record TaskRecord(int Id, string Title, string Status, DateOnly? DueDate, IReadOnlyList<string> Tags, int? OwnerId, DateTimeOffset CreatedAt);

public sealed record NewTask(string Title, string Status = "todo", DateOnly? DueDate = null, IReadOnlyList<string>? Tags = null, int? OwnerId = null);

public sealed record TaskQuery(string? Status = null, string? Tag = null, int Limit = 20, int Offset = 0);

public sealed record Page<T>(IReadOnlyList<T> Items, int Total);

public interface ITaskRepository
{
    Task<TaskRecord> CreateAsync(NewTask task, CancellationToken ct = default);

    Task<TaskRecord?> GetAsync(int id, CancellationToken ct = default);

    Task<Page<TaskRecord>> ListAsync(TaskQuery query, CancellationToken ct = default);

    Task<bool> DeleteAsync(int id, CancellationToken ct = default);
}

/// <summary>Ordered, append-only schema steps; the number applied is stored in PRAGMA user_version.</summary>
public static class TrackerMigrations
{
    private static readonly string[] Steps =
    [
        """
        CREATE TABLE tasks (
            id INTEGER PRIMARY KEY,
            title TEXT NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
            status TEXT NOT NULL DEFAULT 'todo' CHECK (status IN ('todo', 'in_progress', 'done')),
            due_date TEXT,
            owner_id INTEGER,
            created_at TEXT NOT NULL
        );
        CREATE TABLE task_tags (
            task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
            tag TEXT NOT NULL,
            PRIMARY KEY (task_id, tag)
        );
        """,
        "CREATE INDEX ix_tasks_status ON tasks (status)",
    ];

    public static async Task<int> ApplyAsync(SqliteConnection conn, CancellationToken ct = default)
    {
        await using var read = conn.CreateCommand();
        read.CommandText = "PRAGMA user_version";
        var current = (int)(long)(await read.ExecuteScalarAsync(ct))!;

        for (var version = current; version < Steps.Length; version++)
        {
            await using var tx = (SqliteTransaction)await conn.BeginTransactionAsync(ct);
            await using var step = conn.CreateCommand();
            step.Transaction = tx;
            step.CommandText = Steps[version] + $";\nPRAGMA user_version = {version + 1}";
            await step.ExecuteNonQueryAsync(ct);
            await tx.CommitAsync(ct);
        }

        return Steps.Length - current;
    }
}

/// <summary>ADO.NET repository: one short-lived pooled connection per operation.</summary>
public sealed class SqliteTaskRepository(string connectionString, Func<DateTimeOffset> clock) : ITaskRepository
{
    private const string Select = """
        SELECT t.id, t.title, t.status, t.due_date, t.owner_id, t.created_at,
               (SELECT json_group_array(tag) FROM (SELECT tag FROM task_tags WHERE task_id = t.id ORDER BY tag)) AS tags
        FROM tasks t
        """;

    public async Task<TaskRecord> CreateAsync(NewTask task, CancellationToken ct = default)
    {
        await using var conn = await OpenAsync(ct);
        await using var tx = (SqliteTransaction)await conn.BeginTransactionAsync(ct);

        long id;
        await using (var insert = conn.CreateCommand())
        {
            insert.Transaction = tx;
            insert.CommandText = """
                INSERT INTO tasks (title, status, due_date, owner_id, created_at)
                VALUES (@title, @status, @due, @owner, @created) RETURNING id
                """;
            insert.Parameters.AddWithValue("@title", task.Title);
            insert.Parameters.AddWithValue("@status", task.Status);
            insert.Parameters.AddWithValue("@due", task.DueDate?.ToString("yyyy-MM-dd", CultureInfo.InvariantCulture) ?? (object)DBNull.Value);
            insert.Parameters.AddWithValue("@owner", task.OwnerId ?? (object)DBNull.Value);
            insert.Parameters.AddWithValue("@created", clock().UtcDateTime.ToString("yyyy-MM-dd'T'HH:mm:ss'Z'", CultureInfo.InvariantCulture));
            id = (long)(await insert.ExecuteScalarAsync(ct))!;
        }

        foreach (var tag in task.Tags ?? [])
        {
            await using var tagCmd = conn.CreateCommand();
            tagCmd.Transaction = tx;
            tagCmd.CommandText = "INSERT INTO task_tags (task_id, tag) VALUES (@id, @tag)";
            tagCmd.Parameters.AddWithValue("@id", id);
            tagCmd.Parameters.AddWithValue("@tag", tag);
            await tagCmd.ExecuteNonQueryAsync(ct);
        }

        await tx.CommitAsync(ct);
        return (await GetAsync((int)id, ct))!;
    }

    public async Task<TaskRecord?> GetAsync(int id, CancellationToken ct = default)
    {
        await using var conn = await OpenAsync(ct);
        await using var cmd = conn.CreateCommand();
        cmd.CommandText = Select + " WHERE t.id = @id";
        cmd.Parameters.AddWithValue("@id", id);
        await using var reader = await cmd.ExecuteReaderAsync(ct);
        return await reader.ReadAsync(ct) ? Map(reader) : null;
    }

    public async Task<Page<TaskRecord>> ListAsync(TaskQuery query, CancellationToken ct = default)
    {
        // Only fixed SQL fragments are appended; every value travels as a parameter.
        var where = new List<string>();
        var args = new List<SqliteParameter>();
        if (query.Status is not null)
        {
            where.Add("t.status = @status");
            args.Add(new("@status", query.Status));
        }

        if (query.Tag is not null)
        {
            where.Add("EXISTS (SELECT 1 FROM task_tags g WHERE g.task_id = t.id AND g.tag = @tag)");
            args.Add(new("@tag", query.Tag));
        }

        var filter = where.Count == 0 ? "" : " WHERE " + string.Join(" AND ", where);

        await using var conn = await OpenAsync(ct);

        await using var count = conn.CreateCommand();
        count.CommandText = "SELECT count(*) FROM tasks t" + filter;
        foreach (var a in args)
        {
            count.Parameters.Add(new SqliteParameter(a.ParameterName, a.Value));
        }

        var total = (int)(long)(await count.ExecuteScalarAsync(ct))!;

        await using var cmd = conn.CreateCommand();
        cmd.CommandText = Select + filter + " ORDER BY t.created_at DESC, t.id DESC LIMIT @limit OFFSET @offset";
        foreach (var a in args)
        {
            cmd.Parameters.Add(new SqliteParameter(a.ParameterName, a.Value));
        }

        cmd.Parameters.AddWithValue("@limit", query.Limit);
        cmd.Parameters.AddWithValue("@offset", query.Offset);

        var items = new List<TaskRecord>();
        await using var reader = await cmd.ExecuteReaderAsync(ct);
        while (await reader.ReadAsync(ct))
        {
            items.Add(Map(reader));
        }

        return new Page<TaskRecord>(items, total);
    }

    public async Task<bool> DeleteAsync(int id, CancellationToken ct = default)
    {
        await using var conn = await OpenAsync(ct);
        await using var cmd = conn.CreateCommand();
        cmd.CommandText = "DELETE FROM tasks WHERE id = @id";
        cmd.Parameters.AddWithValue("@id", id);
        return await cmd.ExecuteNonQueryAsync(ct) == 1;
    }

    private async Task<SqliteConnection> OpenAsync(CancellationToken ct)
    {
        var conn = new SqliteConnection(connectionString);
        await conn.OpenAsync(ct);
        await using var pragma = conn.CreateCommand();
        pragma.CommandText = "PRAGMA foreign_keys = ON";
        await pragma.ExecuteNonQueryAsync(ct);
        return conn;
    }

    private static TaskRecord Map(SqliteDataReader r) => new(
        r.GetInt32(0),
        r.GetString(1),
        r.GetString(2),
        r.IsDBNull(3) ? null : DateOnly.ParseExact(r.GetString(3), "yyyy-MM-dd", CultureInfo.InvariantCulture),
        JsonSerializer.Deserialize<string[]>(r.GetString(6))!,
        r.IsDBNull(4) ? null : r.GetInt32(4),
        DateTimeOffset.Parse(r.GetString(5), CultureInfo.InvariantCulture, DateTimeStyles.AssumeUniversal));
}

/// <summary>Task Tracker repository end to end, plus the (optional) PostgreSQL swap.</summary>
public static class TaskRepositoryDemo
{
    public static async Task<int> RunAsync(string[] args)
    {
        var connectionString = new SqliteConnectionStringBuilder
        {
            DataSource = "tracker-demo",
            Mode = SqliteOpenMode.Memory,
            Cache = SqliteCacheMode.Shared,
        }.ToString();

        // Shared in-memory databases vanish when the last connection closes, so keep one open.
        await using var keeper = new SqliteConnection(connectionString);
        await keeper.OpenAsync();
        Console.WriteLine($"migrations applied: {await TrackerMigrations.ApplyAsync(keeper)}");
        Console.WriteLine($"migrations applied: {await TrackerMigrations.ApplyAsync(keeper)}");

        var tick = 0;
        ITaskRepository repo = new SqliteTaskRepository(connectionString, () => new DateTimeOffset(2030, 1, 1, 9, 0, 0, TimeSpan.Zero).AddMinutes(tick++));

        await repo.CreateAsync(new NewTask("Write docs", DueDate: new DateOnly(2030, 1, 31), Tags: ["docs", "p1"]));
        await repo.CreateAsync(new NewTask("Fix bug", "in_progress", Tags: ["bug"], OwnerId: 7));
        var third = await repo.CreateAsync(new NewTask("Release", Tags: ["p1"]));

        var page = await repo.ListAsync(new TaskQuery(Limit: 2));
        Console.WriteLine($"page of {page.Items.Count}/{page.Total}: {string.Join(", ", page.Items.Select(t => t.Title))}");

        var p1 = await repo.ListAsync(new TaskQuery(Tag: "p1"));
        Console.WriteLine($"tag p1: {string.Join(", ", p1.Items.Select(t => t.Title))}");

        var inProgress = await repo.ListAsync(new TaskQuery(Status: "in_progress"));
        var t = inProgress.Items[0];
        Console.WriteLine($"in_progress: #{t.Id} {t.Title} due={t.DueDate?.ToString("O") ?? "null"} tags=[{string.Join(',', t.Tags)}] owner={t.OwnerId} created={t.CreatedAt:yyyy-MM-ddTHH:mm:ssZ}");

        Console.WriteLine($"deleted {third.Id}: {await repo.DeleteAsync(third.Id)}, again: {await repo.DeleteAsync(third.Id)}");
        Console.WriteLine($"get {third.Id}: {(await repo.GetAsync(third.Id))?.ToString() ?? "null"}");

        await PostgresSwap.TryAsync();
        return 0;
    }
}
