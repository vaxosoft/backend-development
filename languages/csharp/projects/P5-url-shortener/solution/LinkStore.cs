using Microsoft.Data.Sqlite;

namespace P5.UrlShortener;

/// <summary>SQL access for links. Every method uses parameters (<c>@name</c>), never string concatenation.</summary>
public sealed class LinkStore(Database database, TimeProvider time)
{
    private const int SqliteConstraint = 19;

    /// <summary>Inserts a link; false when the slug already exists (primary key violation).</summary>
    public async Task<bool> CreateAsync(string slug, string url, long ownerId, CancellationToken cancellationToken)
    {
        await using var connection = database.Open();
        await using var command = connection.CreateCommand();
        command.CommandText = "INSERT INTO links (slug, url, owner_id, created_at) VALUES (@slug, @url, @owner, @created)";
        command.Parameters.AddWithValue("@slug", slug);
        command.Parameters.AddWithValue("@url", url);
        command.Parameters.AddWithValue("@owner", ownerId);
        command.Parameters.AddWithValue("@created", time.GetUtcNow().ToString("O"));
        try
        {
            await command.ExecuteNonQueryAsync(cancellationToken);
            return true;
        }
        catch (SqliteException e) when (e.SqliteErrorCode == SqliteConstraint)
        {
            return false;
        }
    }

    /// <summary>Atomically increments the hit counter and returns the target URL, or null for an unknown slug.</summary>
    public async Task<string?> HitAsync(string slug, CancellationToken cancellationToken)
    {
        await using var connection = database.Open();
        await using var command = connection.CreateCommand();
        command.CommandText = "UPDATE links SET hits = hits + 1 WHERE slug = @slug RETURNING url";
        command.Parameters.AddWithValue("@slug", slug);
        return (string?)await command.ExecuteScalarAsync(cancellationToken);
    }

    public async Task<LinkRow?> FindAsync(string slug, CancellationToken cancellationToken)
    {
        await using var connection = database.Open();
        await using var command = connection.CreateCommand();
        command.CommandText = "SELECT slug, url, owner_id FROM links WHERE slug = @slug";
        command.Parameters.AddWithValue("@slug", slug);
        await using var reader = await command.ExecuteReaderAsync(cancellationToken);
        return await reader.ReadAsync(cancellationToken)
            ? new LinkRow(reader.GetString(0), reader.GetString(1), reader.GetInt64(2))
            : null;
    }

    /// <summary>The owner's links, oldest first, with their hit counts.</summary>
    public async Task<IReadOnlyList<LinkSummary>> ListByOwnerAsync(long ownerId, CancellationToken cancellationToken)
    {
        await using var connection = database.Open();
        await using var command = connection.CreateCommand();
        command.CommandText = "SELECT slug, url, hits FROM links WHERE owner_id = @owner ORDER BY created_at, rowid";
        command.Parameters.AddWithValue("@owner", ownerId);
        await using var reader = await command.ExecuteReaderAsync(cancellationToken);
        var links = new List<LinkSummary>();
        while (await reader.ReadAsync(cancellationToken))
        {
            links.Add(new LinkSummary(reader.GetString(0), reader.GetString(1), reader.GetInt32(2)));
        }

        return links;
    }

    public async Task<bool> DeleteAsync(string slug, CancellationToken cancellationToken)
    {
        await using var connection = database.Open();
        await using var command = connection.CreateCommand();
        command.CommandText = "DELETE FROM links WHERE slug = @slug";
        command.Parameters.AddWithValue("@slug", slug);
        return await command.ExecuteNonQueryAsync(cancellationToken) > 0;
    }
}
