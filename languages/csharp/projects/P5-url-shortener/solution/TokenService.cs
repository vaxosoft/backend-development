using System.Security.Claims;
using Microsoft.Extensions.Options;
using Microsoft.IdentityModel.JsonWebTokens;
using Microsoft.IdentityModel.Tokens;

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
        var now = time.GetUtcNow().UtcDateTime;
        var descriptor = new SecurityTokenDescriptor
        {
            Issuer = ShortenerOptions.Issuer,
            Audience = ShortenerOptions.Audience,
            Subject = new ClaimsIdentity(
            [
                new Claim(JwtRegisteredClaimNames.Sub, userId.ToString()),
                new Claim(JwtRegisteredClaimNames.Email, email),
            ]),
            IssuedAt = now,
            NotBefore = now,
            Expires = now + Lifetime,
            SigningCredentials = new SigningCredentials(options.Value.SigningKey, SecurityAlgorithms.HmacSha256),
        };
        return new JsonWebTokenHandler().CreateToken(descriptor);
    }
}
