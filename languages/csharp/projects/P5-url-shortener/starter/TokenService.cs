using Microsoft.Extensions.Options;

namespace P5.UrlShortener;

/// <summary>Issues HS256 JWTs for logged-in users.</summary>
public sealed class TokenService(IOptions<ShortenerOptions> options, TimeProvider time)
{
    public static readonly TimeSpan Lifetime = TimeSpan.FromHours(1);

    /// <summary>
    /// Creates a signed token with issuer/audience from <see cref="ShortenerOptions"/>, the claims
    /// <c>sub</c> (user id) and <c>email</c>, <c>iat</c>/<c>nbf</c> = now and <c>exp</c> = now + <see cref="Lifetime"/>.
    /// Use <paramref name="time"/>, not <c>DateTime.UtcNow</c>.
    /// </summary>
    public string Issue(long userId, string email)
    {
        // TODO(P5): SecurityTokenDescriptor + JsonWebTokenHandler.CreateToken, signed with options.Value.SigningKey (HMAC-SHA256).
        _ = (options, time, userId, email);
        throw new NotImplementedException("TODO(P5): TokenService.Issue");
    }
}
