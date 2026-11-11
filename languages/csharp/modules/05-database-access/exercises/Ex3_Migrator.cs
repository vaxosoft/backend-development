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
    public static Task<IReadOnlyList<int>> MigrateAsync(SqliteConnection conn, IReadOnlyList<Migration> migrations, CancellationToken ct = default) =>
        throw new NotImplementedException();

    /// <summary>The highest applied version, or 0 when nothing was applied (also when the bookkeeping table does not exist yet).</summary>
    public static Task<int> CurrentVersionAsync(SqliteConnection conn, CancellationToken ct = default) =>
        throw new NotImplementedException();
}
