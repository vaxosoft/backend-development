using Microsoft.Data.Sqlite;

namespace M05.Exercises;

public sealed record User(long Id, string Name, string Email, string? Bio);

public static class Ex1_UserStore
{
    /// <summary>Insert a user (parameters only, null bio stored as SQL NULL) and return it with its new id.</summary>
    public static Task<User> AddAsync(SqliteConnection conn, string name, string email, string? bio, CancellationToken ct = default) =>
        throw new NotImplementedException();

    /// <summary>The user with this exact e-mail, or null. Must be safe against SQL injection.</summary>
    public static Task<User?> FindByEmailAsync(SqliteConnection conn, string email, CancellationToken ct = default) =>
        throw new NotImplementedException();

    /// <summary>All users whose name contains <paramref name="fragment"/> (case-insensitive), ordered by id. The fragment is plain text, not a LIKE pattern.</summary>
    public static Task<IReadOnlyList<User>> SearchByNameAsync(SqliteConnection conn, string fragment, CancellationToken ct = default) =>
        throw new NotImplementedException();
}
