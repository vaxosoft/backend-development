using System.Buffers.Text;
using System.Security.Cryptography;
using System.Text;

namespace M13.Examples;

public static class IdsRandomHash
{
    public static Task<int> RunAsync(string[] args)
    {
        var random = Guid.NewGuid();
        Console.WriteLine($"Guid.NewGuid() is version {random.Version}");

        // Version 7 = 48-bit Unix millisecond timestamp first, so ids sort by creation time.
        var at = DateTimeOffset.FromUnixTimeMilliseconds(0x0123456789AB);
        var v7 = Guid.CreateVersion7(at);
        Console.WriteLine($"v7 at fixed time: {v7.ToString()[..15]}...");
        var later = Guid.CreateVersion7(at.AddMilliseconds(1));
        Console.WriteLine($"later id sorts after (ordinal string order): {string.CompareOrdinal(v7.ToString(), later.ToString()) < 0}");

        var a = new Random(42);
        var b = new Random(42);
        Console.WriteLine($"seeded Random repeats: {a.Next(1000) == b.Next(1000)}");

        var dice = RandomNumberGenerator.GetInt32(1, 7);
        Console.WriteLine($"crypto random in range: {dice is >= 1 and <= 6}");
        Console.WriteLine($"random token length: {Base64Url.EncodeToString(RandomNumberGenerator.GetBytes(24)).Length}");

        var sha = SHA256.HashData(Encoding.UTF8.GetBytes("abc"));
        Console.WriteLine($"sha256(abc) = {Convert.ToHexStringLower(sha)}");

        var mac = HMACSHA256.HashData(Encoding.UTF8.GetBytes("key"), Encoding.UTF8.GetBytes("The quick brown fox jumps over the lazy dog"));
        Console.WriteLine($"hmac-sha256  = {Convert.ToHexStringLower(mac)}");
        Console.WriteLine($"constant-time compare: {CryptographicOperations.FixedTimeEquals(mac, mac.ToArray())}");
        return Task.FromResult(0);
    }
}
