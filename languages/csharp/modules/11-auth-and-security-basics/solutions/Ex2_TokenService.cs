using System.Security.Claims;
using System.Text;
using Microsoft.IdentityModel.JsonWebTokens;
using Microsoft.IdentityModel.Tokens;

namespace M11.Exercises;

public sealed record TokenOptions(string Issuer, string Audience, string Secret, TimeSpan Lifetime);

public sealed record AuthenticatedUser(string UserId, string Role);

/// <summary>
/// Issues and validates HS256 JWTs. The <c>sub</c> claim carries the user id and <c>role</c> the role.
/// All time decisions come from the injected <see cref="TimeProvider"/>.
/// </summary>
public sealed class Ex2_TokenService
{
    private readonly TokenOptions _options;
    private readonly TimeProvider _time;
    private readonly SymmetricSecurityKey _key;
    private readonly JsonWebTokenHandler _handler = new();
    private readonly TokenValidationParameters _parameters;

    public Ex2_TokenService(TokenOptions options, TimeProvider time)
    {
        var secret = Encoding.UTF8.GetBytes(options.Secret);
        if (secret.Length < 32)
        {
            throw new ArgumentException("secret must be at least 32 bytes", nameof(options));
        }

        _options = options;
        _time = time;
        _key = new SymmetricSecurityKey(secret);
        _parameters = new TokenValidationParameters
        {
            ValidIssuer = options.Issuer,
            ValidAudience = options.Audience,
            IssuerSigningKey = _key,
            ValidAlgorithms = [SecurityAlgorithms.HmacSha256],
            RequireExpirationTime = true,
            LifetimeValidator = (notBefore, expires, _, _) =>
            {
                var now = _time.GetUtcNow().UtcDateTime;
                return expires > now && (notBefore is null || notBefore <= now);
            },
            NameClaimType = "sub",
            RoleClaimType = "role",
        };
    }

    public string Issue(string userId, string role)
    {
        var now = _time.GetUtcNow().UtcDateTime;
        return _handler.CreateToken(new SecurityTokenDescriptor
        {
            Issuer = _options.Issuer,
            Audience = _options.Audience,
            IssuedAt = now,
            NotBefore = now,
            Expires = now + _options.Lifetime,
            Subject = new ClaimsIdentity([new Claim("sub", userId), new Claim("role", role)]),
            SigningCredentials = new SigningCredentials(_key, SecurityAlgorithms.HmacSha256),
        });
    }

    /// <summary>Returns null for any invalid token: bad signature, expired, wrong issuer/audience/algorithm, garbage.</summary>
    public AuthenticatedUser? Validate(string token)
    {
        var result = _handler.ValidateTokenAsync(token, _parameters).GetAwaiter().GetResult();
        if (!result.IsValid)
        {
            return null;
        }

        var identity = result.ClaimsIdentity;
        return new AuthenticatedUser(identity.Name!, identity.FindFirst("role")!.Value);
    }
}
