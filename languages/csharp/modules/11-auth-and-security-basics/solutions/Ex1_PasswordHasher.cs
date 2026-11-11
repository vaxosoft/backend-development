using System.Security.Cryptography;

namespace M11.Exercises;

/// <summary>
/// Hashes passwords as <c>pbkdf2-sha256$&lt;iterations&gt;$&lt;salt-base64&gt;$&lt;hash-base64&gt;</c>
/// (16-byte random salt, 32-byte hash).
/// </summary>
public sealed class Ex1_PasswordHasher(int iterations = 600_000)
{
    private const string Prefix = "pbkdf2-sha256";
    private const int SaltSize = 16;
    private const int HashSize = 32;

    public string Hash(string password)
    {
        ArgumentNullException.ThrowIfNull(password);
        var salt = RandomNumberGenerator.GetBytes(SaltSize);
        var hash = Rfc2898DeriveBytes.Pbkdf2(password, salt, iterations, HashAlgorithmName.SHA256, HashSize);
        return $"{Prefix}${iterations}${Convert.ToBase64String(salt)}${Convert.ToBase64String(hash)}";
    }

    /// <summary>Never throws for malformed records; returns false instead.</summary>
    public bool Verify(string password, string stored)
    {
        if (!TryParse(stored, out var count, out var salt, out var expected))
        {
            return false;
        }

        var actual = Rfc2898DeriveBytes.Pbkdf2(password, salt, count, HashAlgorithmName.SHA256, expected.Length);
        return CryptographicOperations.FixedTimeEquals(actual, expected);
    }

    /// <summary>True when the record is malformed or uses fewer iterations than this hasher's setting.</summary>
    public bool NeedsRehash(string stored) =>
        !TryParse(stored, out var count, out _, out _) || count < iterations;

    private static bool TryParse(string stored, out int count, out byte[] salt, out byte[] hash)
    {
        count = 0;
        salt = [];
        hash = [];
        return stored.Split('$') is [Prefix, var iterText, var saltText, var hashText]
            && int.TryParse(iterText, out count) && count > 0
            && TryDecode(saltText, out salt)
            && TryDecode(hashText, out hash)
            && hash.Length > 0;
    }

    private static bool TryDecode(string text, out byte[] bytes)
    {
        bytes = new byte[text.Length];
        if (Convert.TryFromBase64String(text, bytes, out var written))
        {
            bytes = bytes[..written];
            return true;
        }

        return false;
    }
}
