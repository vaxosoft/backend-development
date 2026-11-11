namespace P5.UrlShortener;

/// <summary>SQL access for users. Every method uses parameters (<c>@name</c>), never string concatenation.</summary>
public sealed class UserStore(Database database)
{
    /// <summary>Inserts a user; returns the new id, or null when the email is already registered (UNIQUE violation).</summary>
    public Task<long?> CreateAsync(string email, string passwordHash, CancellationToken cancellationToken)
    {
        // TODO(P5): INSERT ... ; catch SqliteException with SqliteErrorCode 19 (constraint) -> null.
        _ = (database, email, passwordHash, cancellationToken);
        throw new NotImplementedException("TODO(P5): UserStore.CreateAsync");
    }

    public Task<User?> FindByEmailAsync(string email, CancellationToken cancellationToken)
    {
        // TODO(P5): SELECT id, email, password_hash FROM users WHERE email = @email
        _ = (email, cancellationToken);
        throw new NotImplementedException("TODO(P5): UserStore.FindByEmailAsync");
    }
}
