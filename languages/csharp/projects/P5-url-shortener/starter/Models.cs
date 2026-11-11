namespace P5.UrlShortener;

public sealed record Credentials(string? Email, string? Password);

public sealed record CreateLinkRequest(string? Url, string? Slug);

public sealed record TokenResponse(string Token);

public sealed record RegisteredUser(long Id, string Email);

public sealed record LinkResponse(string Slug, string Url);

public sealed record LinkSummary(string Slug, string Url, int Hits);

public sealed record LinkList(IReadOnlyList<LinkSummary> Items, int Total);

public sealed record User(long Id, string Email, string PasswordHash);

public sealed record LinkRow(string Slug, string Url, long OwnerId);
