namespace P5.UrlShortener;

/// <summary>SQL access for links. Every method uses parameters (<c>@name</c>), never string concatenation.</summary>
public sealed class LinkStore(Database database, TimeProvider time)
{
    /// <summary>Inserts a link; false when the slug already exists (primary key violation).</summary>
    public Task<bool> CreateAsync(string slug, string url, long ownerId, CancellationToken cancellationToken)
    {
        // TODO(P5): INSERT ... with created_at = time.GetUtcNow() as ISO 8601; SqliteException with SqliteErrorCode 19 -> false.
        _ = (database, time, slug, url, ownerId, cancellationToken);
        throw new NotImplementedException("TODO(P5): LinkStore.CreateAsync");
    }

    /// <summary>Atomically increments the hit counter and returns the target URL, or null for an unknown slug.</summary>
    public Task<string?> HitAsync(string slug, CancellationToken cancellationToken)
    {
        // TODO(P5): one statement: UPDATE links SET hits = hits + 1 WHERE slug = @slug RETURNING url
        _ = (slug, cancellationToken);
        throw new NotImplementedException("TODO(P5): LinkStore.HitAsync");
    }

    public Task<LinkRow?> FindAsync(string slug, CancellationToken cancellationToken)
    {
        // TODO(P5): SELECT slug, url, owner_id FROM links WHERE slug = @slug
        _ = (slug, cancellationToken);
        throw new NotImplementedException("TODO(P5): LinkStore.FindAsync");
    }

    /// <summary>The owner's links, oldest first, with their hit counts.</summary>
    public Task<IReadOnlyList<LinkSummary>> ListByOwnerAsync(long ownerId, CancellationToken cancellationToken)
    {
        // TODO(P5): SELECT slug, url, hits ... WHERE owner_id = @owner ORDER BY created_at, rowid
        _ = (ownerId, cancellationToken);
        throw new NotImplementedException("TODO(P5): LinkStore.ListByOwnerAsync");
    }

    public Task<bool> DeleteAsync(string slug, CancellationToken cancellationToken)
    {
        // TODO(P5): DELETE ... WHERE slug = @slug; true when a row was removed.
        _ = (slug, cancellationToken);
        throw new NotImplementedException("TODO(P5): LinkStore.DeleteAsync");
    }
}
