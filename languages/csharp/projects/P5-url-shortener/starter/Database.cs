using Microsoft.Data.Sqlite;
using Microsoft.Extensions.Options;

namespace P5.UrlShortener;

/// <summary>Opens SQLite connections and creates the schema once at startup.</summary>
public sealed class Database : IDisposable
{
    private readonly string _connectionString;
    // Keeps a shared in-memory database alive between per-operation connections.
    private readonly SqliteConnection _keepAlive;

    public Database(IOptions<ShortenerOptions> options)
    {
        _connectionString = options.Value.ConnectionString;
        _keepAlive = Open();
        using var command = _keepAlive.CreateCommand();
        command.CommandText = """
            CREATE TABLE IF NOT EXISTS users (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                email TEXT NOT NULL UNIQUE,
                password_hash TEXT NOT NULL
            );
            CREATE TABLE IF NOT EXISTS links (
                slug TEXT PRIMARY KEY,
                url TEXT NOT NULL,
                owner_id INTEGER NOT NULL REFERENCES users(id),
                hits INTEGER NOT NULL DEFAULT 0,
                created_at TEXT NOT NULL
            );
            """;
        command.ExecuteNonQuery();
    }

    /// <summary>A new open connection; the caller disposes it.</summary>
    public SqliteConnection Open()
    {
        var connection = new SqliteConnection(_connectionString);
        connection.Open();
        return connection;
    }

    public void Dispose() => _keepAlive.Dispose();
}
