#pragma warning disable CS9113 // starter: your implementation will read these parameters

namespace M11.Exercises;

/// <summary>
/// Hashes passwords as <c>pbkdf2-sha256$&lt;iterations&gt;$&lt;salt-base64&gt;$&lt;hash-base64&gt;</c>
/// (16-byte random salt, 32-byte hash).
/// </summary>
public sealed class Ex1_PasswordHasher(int iterations = 600_000)
{
    public string Hash(string password) => throw new NotImplementedException();

    /// <summary>Never throws for malformed records; returns false instead.</summary>
    public bool Verify(string password, string stored) => throw new NotImplementedException();

    /// <summary>True when the record is malformed or uses fewer iterations than this hasher's setting.</summary>
    public bool NeedsRehash(string stored) => throw new NotImplementedException();
}
