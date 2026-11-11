using Microsoft.Data.Sqlite;

namespace M05.Exercises;

public sealed record Migration(int Version, string Name, string Sql);

public static class Ex3_Migrator
{
    /// <summary>
    /// Bring the database up to date and return the versions applied by THIS call, in order.
    /// <list type="bullet">
    /// <item>Applied versions live in <c>schema_migrations(version INTEGER PRIMARY KEY, name TEXT NOT NULL)</c>, created on demand.</item>
    /// <item>Pending migrations run in ascending version order, each in its own transaction together with its bookkeeping row.</item>
    /// <item>A failing migration rolls back only itself; earlier ones stay applied and the exception propagates.</item>
    /// <item>Duplicate versions in the input: ArgumentException before anything runs.</item>
    /// <item>A pending version lower than the highest applied one: InvalidOperationException (history was rewritten).</item>
    /// <item>Running again with nothing pending returns an empty list.</item>
    /// </list>
    /// </summary>
    public static async Task<IReadOnlyList<int>> MigrateAsync(SqliteConnection conn, IReadOnlyList<Migration> migrations, CancellationToken ct = default)
    {
        if (migrations.GroupBy(m => m.Version).Any(g => g.Count() > 1))
        {
            throw new ArgumentException("duplicate migration versions", nameof(migrations));
        }

        await ExecuteAsync(conn, null, "CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL)", ct);
        var current = await CurrentVersionAsync(conn, ct);
        var applied = await AppliedVersionsAsync(conn, ct);

        var pending = migrations.Where(m => !applied.Contains(m.Version)).OrderBy(m => m.Version).ToList();
        if (pending.Any(m => m.Version < current))
        {
            throw new InvalidOperationException("a pending migration is older than the latest applied one");
        }

        var done = new List<int>();
        foreach (var migration in pending)
        {
            await using var tx = (SqliteTransaction)await conn.BeginTransactionAsync(ct);
            await ExecuteAsync(conn, tx, migration.Sql, ct);

            await using var record = conn.CreateCommand();
            record.Transaction = tx;
            record.CommandText = "INSERT INTO schema_migrations (version, name) VALUES (@version, @name)";
            record.Parameters.AddWithValue("@version", migration.Version);
            record.Parameters.AddWithValue("@name", migration.Name);
            await record.ExecuteNonQueryAsync(ct);

            await tx.CommitAsync(ct);
            done.Add(migration.Version);
        }

        return done;
    }

    /// <summary>The highest applied version, or 0 when nothing was applied (also when the bookkeeping table does not exist yet).</summary>
    public static async Task<int> CurrentVersionAsync(SqliteConnection conn, CancellationToken ct = default)
    {
        await using var exists = conn.CreateCommand();
        exists.CommandText = "SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'schema_migrations'";
        if ((long)(await exists.ExecuteScalarAsync(ct))! == 0)
        {
            return 0;
        }

        await using var cmd = conn.CreateCommand();
        cmd.CommandText = "SELECT COALESCE(MAX(version), 0) FROM schema_migrations";
        return (int)(long)(await cmd.ExecuteScalarAsync(ct))!;
    }

    private static async Task<HashSet<int>> AppliedVersionsAsync(SqliteConnection conn, CancellationToken ct)
    {
        await using var cmd = conn.CreateCommand();
        cmd.CommandText = "SELECT version FROM schema_migrations";
        var versions = new HashSet<int>();
        await using var reader = await cmd.ExecuteReaderAsync(ct);
        while (await reader.ReadAsync(ct))
        {
            versions.Add(reader.GetInt32(0));
        }

        return versions;
    }

    private static async Task ExecuteAsync(SqliteConnection conn, SqliteTransaction? tx, string sql, CancellationToken ct)
    {
        await using var cmd = conn.CreateCommand();
        cmd.Transaction = tx;
        cmd.CommandText = sql;
        await cmd.ExecuteNonQueryAsync(ct);
    }
}
