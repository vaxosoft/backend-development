using Microsoft.Data.Sqlite;

namespace P5.UrlShortener;

/// <summary>SQL access for users. Every method uses parameters (<c>@name</c>), never string concatenation.</summary>
public sealed class UserStore(Database database)
{
    private const int SqliteConstraint = 19;

    /// <summary>Inserts a user; returns the new id, or null when the email is already registered (UNIQUE violation).</summary>
    public async Task<long?> CreateAsync(string email, string passwordHash, CancellationToken cancellationToken)
    {
        await using var connection = database.Open();
        await using var command = connection.CreateCommand();
        command.CommandText = "INSERT INTO users (email, password_hash) VALUES (@email, @hash) RETURNING id";
        command.Parameters.AddWithValue("@email", email);
        command.Parameters.AddWithValue("@hash", passwordHash);
        try
        {
            return (long?)await command.ExecuteScalarAsync(cancellationToken);
        }
        catch (SqliteException e) when (e.SqliteErrorCode == SqliteConstraint)
        {
            return null;
        }
    }

    public async Task<User?> FindByEmailAsync(string email, CancellationToken cancellationToken)
    {
        await using var connection = database.Open();
        await using var command = connection.CreateCommand();
        command.CommandText = "SELECT id, email, password_hash FROM users WHERE email = @email";
        command.Parameters.AddWithValue("@email", email);
        await using var reader = await command.ExecuteReaderAsync(cancellationToken);
        return await reader.ReadAsync(cancellationToken)
            ? new User(reader.GetInt64(0), reader.GetString(1), reader.GetString(2))
            : null;
    }
}
