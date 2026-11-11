using Microsoft.Data.Sqlite;

namespace M05.Exercises;

public sealed record User(long Id, string Name, string Email, string? Bio);

public static class Ex1_UserStore
{
    private const string Columns = "id, name, email, bio";

    /// <summary>Insert a user (parameters only, null bio stored as SQL NULL) and return it with its new id.</summary>
    public static async Task<User> AddAsync(SqliteConnection conn, string name, string email, string? bio, CancellationToken ct = default)
    {
        await using var cmd = conn.CreateCommand();
        cmd.CommandText = "INSERT INTO users (name, email, bio) VALUES (@name, @email, @bio) RETURNING id";
        cmd.Parameters.AddWithValue("@name", name);
        cmd.Parameters.AddWithValue("@email", email);
        cmd.Parameters.AddWithValue("@bio", (object?)bio ?? DBNull.Value);
        var id = (long)(await cmd.ExecuteScalarAsync(ct))!;
        return new User(id, name, email, bio);
    }

    /// <summary>The user with this exact e-mail, or null. Must be safe against SQL injection.</summary>
    public static async Task<User?> FindByEmailAsync(SqliteConnection conn, string email, CancellationToken ct = default)
    {
        await using var cmd = conn.CreateCommand();
        cmd.CommandText = $"SELECT {Columns} FROM users WHERE email = @email";
        cmd.Parameters.AddWithValue("@email", email);
        await using var reader = await cmd.ExecuteReaderAsync(ct);
        return await reader.ReadAsync(ct) ? Map(reader) : null;
    }

    /// <summary>All users whose name contains <paramref name="fragment"/> (case-insensitive), ordered by id. The fragment is plain text, not a LIKE pattern.</summary>
    public static async Task<IReadOnlyList<User>> SearchByNameAsync(SqliteConnection conn, string fragment, CancellationToken ct = default)
    {
        await using var cmd = conn.CreateCommand();
        cmd.CommandText = $"SELECT {Columns} FROM users WHERE instr(lower(name), lower(@fragment)) > 0 ORDER BY id";
        cmd.Parameters.AddWithValue("@fragment", fragment);
        var users = new List<User>();
        await using var reader = await cmd.ExecuteReaderAsync(ct);
        while (await reader.ReadAsync(ct))
        {
            users.Add(Map(reader));
        }

        return users;
    }

    private static User Map(SqliteDataReader r) =>
        new(r.GetInt64(0), r.GetString(1), r.GetString(2), r.IsDBNull(3) ? null : r.GetString(3));
}
