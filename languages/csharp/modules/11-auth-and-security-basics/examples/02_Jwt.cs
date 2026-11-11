using System.Security.Claims;
using System.Text;
using Microsoft.IdentityModel.JsonWebTokens;
using Microsoft.IdentityModel.Tokens;

namespace M11.Examples;

public static class Jwt
{
    private const string Issuer = "tracker";
    private const string Audience = "tracker-api";
    private static readonly SymmetricSecurityKey Key = new(Encoding.UTF8.GetBytes("a-demo-secret-that-is-at-least-32-bytes!"));

    private static TokenValidationParameters Parameters(SecurityKey key) => new()
    {
        ValidIssuer = Issuer,
        ValidAudience = Audience,
        IssuerSigningKey = key,
        ValidAlgorithms = [SecurityAlgorithms.HmacSha256],
        ClockSkew = TimeSpan.Zero,
        NameClaimType = "sub",
        RoleClaimType = "role",
    };

    private static string Create(SecurityKey key, DateTime notBefore, DateTime expires) =>
        new JsonWebTokenHandler().CreateToken(new SecurityTokenDescriptor
        {
            Issuer = Issuer,
            Audience = Audience,
            NotBefore = notBefore,
            Expires = expires,
            Subject = new ClaimsIdentity([new Claim("sub", "42"), new Claim("role", "admin")]),
            SigningCredentials = new SigningCredentials(key, SecurityAlgorithms.HmacSha256),
        });

    private static async Task Report(string label, string token, TokenValidationParameters parameters)
    {
        var result = await new JsonWebTokenHandler().ValidateTokenAsync(token, parameters);
        var detail = result.IsValid
            ? $"sub={result.ClaimsIdentity.Name} admin={result.ClaimsIdentity.HasClaim("role", "admin")}"
            : result.Exception!.GetType().Name;
        Console.WriteLine($"{label,-14} valid={result.IsValid,-5} {detail}");
    }

    public static async Task<int> RunAsync(string[] args)
    {
        var now = DateTime.UtcNow;
        var good = Create(Key, now.AddMinutes(-1), now.AddHours(1));

        var parts = good.Split('.');
        Console.WriteLine($"header: {Encoding.UTF8.GetString(Base64UrlEncoder.DecodeBytes(parts[0]))}");
        Console.WriteLine($"claims: {string.Join(",", new JsonWebToken(good).Claims.Select(c => c.Type).Order())}");

        await Report("valid", good, Parameters(Key));
        await Report("expired", Create(Key, now.AddHours(-2), now.AddHours(-1)), Parameters(Key));

        var otherKey = new SymmetricSecurityKey(Encoding.UTF8.GetBytes("another-secret-that-is-also-32-bytes!!"));
        await Report("wrong key", Create(otherKey, now.AddMinutes(-1), now.AddHours(1)), Parameters(Key));

        var forgedPayload = Base64UrlEncoder.Encode("""{"sub":"1","role":"admin"}""");
        await Report("tampered", $"{parts[0]}.{forgedPayload}.{parts[2]}", Parameters(Key));

        var otherAudience = Parameters(Key);
        otherAudience.ValidAudience = "someone-else";
        await Report("wrong audience", good, otherAudience);
        return 0;
    }
}
