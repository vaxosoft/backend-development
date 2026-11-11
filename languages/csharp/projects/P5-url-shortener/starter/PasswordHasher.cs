using Microsoft.Extensions.Options;

namespace P5.UrlShortener;

/// <summary>Salted PBKDF2 password hashing. Stored format: <c>pbkdf2-sha256$iterations$salt$hash</c> (base64).</summary>
public sealed class PasswordHasher(IOptions<ShortenerOptions> options)
{
    private readonly int _iterations = options.Value.Pbkdf2Iterations;

    /// <summary>Hashes with a fresh random 16-byte salt and a 32-byte PBKDF2-HMAC-SHA256 key.</summary>
    public string Hash(string password)
    {
        // TODO(P5): Rfc2898DeriveBytes.Pbkdf2 + RandomNumberGenerator, never store the plain password.
        _ = (password, _iterations);
        throw new NotImplementedException("TODO(P5): PasswordHasher.Hash");
    }

    /// <summary>Checks a password against a stored value in constant time. Malformed stored values return false.</summary>
    public bool Verify(string password, string stored)
    {
        // TODO(P5): re-derive with the stored salt/iterations and compare with CryptographicOperations.FixedTimeEquals.
        _ = (password, stored);
        throw new NotImplementedException("TODO(P5): PasswordHasher.Verify");
    }
}
