using System.Text;
using Microsoft.IdentityModel.Tokens;

namespace P5.UrlShortener;

/// <summary>Settings bound from the <c>Shortener</c> configuration section (env: <c>Shortener__JwtSecret</c>, ...).</summary>
public sealed class ShortenerOptions
{
    public const string SectionName = "Shortener";
    public const string Issuer = "p5-url-shortener";
    public const string Audience = "p5-url-shortener-api";

    /// <summary>SQLite connection string. Tests use a shared in-memory database.</summary>
    public string ConnectionString { get; set; } = "Data Source=shortener.db";

    /// <summary>HS256 signing secret, at least 32 characters. Required; never put it in appsettings.json.</summary>
    public string JwtSecret { get; set; } = "";

    public int Pbkdf2Iterations { get; set; } = 600_000;

    /// <summary>Token bucket capacity for <c>POST /links</c> (per user).</summary>
    public int RateLimitBurst { get; set; } = 10;

    /// <summary>Tokens added to the bucket per second (may be fractional).</summary>
    public double RateLimitRefillPerSecond { get; set; } = 1;

    public SymmetricSecurityKey SigningKey => new(Encoding.UTF8.GetBytes(JwtSecret));
}
