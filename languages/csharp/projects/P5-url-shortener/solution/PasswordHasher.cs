using System.Security.Cryptography;
using Microsoft.Extensions.Options;

namespace P5.UrlShortener;

/// <summary>Salted PBKDF2 password hashing. Stored format: <c>pbkdf2-sha256$iterations$salt$hash</c> (base64).</summary>
public sealed class PasswordHasher(IOptions<ShortenerOptions> options)
{
    private const string Prefix = "pbkdf2-sha256";
    private const int SaltSize = 16;
    private const int KeySize = 32;
    private readonly int _iterations = options.Value.Pbkdf2Iterations;

    /// <summary>Hashes with a fresh random 16-byte salt and a 32-byte PBKDF2-HMAC-SHA256 key.</summary>
    public string Hash(string password)
    {
        var salt = RandomNumberGenerator.GetBytes(SaltSize);
        var key = Rfc2898DeriveBytes.Pbkdf2(password, salt, _iterations, HashAlgorithmName.SHA256, KeySize);
        return $"{Prefix}${_iterations}${Convert.ToBase64String(salt)}${Convert.ToBase64String(key)}";
    }

    /// <summary>Checks a password against a stored value in constant time. Malformed stored values return false.</summary>
    public bool Verify(string password, string stored)
    {
        var parts = stored.Split('$');
        if (parts.Length != 4 || parts[0] != Prefix || !int.TryParse(parts[1], out var iterations) || iterations <= 0)
        {
            return false;
        }

        try
        {
            var salt = Convert.FromBase64String(parts[2]);
            var expected = Convert.FromBase64String(parts[3]);
            var actual = Rfc2898DeriveBytes.Pbkdf2(password, salt, iterations, HashAlgorithmName.SHA256, expected.Length);
            return CryptographicOperations.FixedTimeEquals(actual, expected);
        }
        catch (FormatException)
        {
            return false;
        }
    }
}
