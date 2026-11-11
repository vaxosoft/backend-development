using System.Buffers.Text;
using System.IO.Compression;
using System.Security.Cryptography;
using System.Text;

namespace M08.Examples;

/// <summary>Example 4: gzip and zip, Base64/Base64Url/hex, and hashing a stream.</summary>
public static class CompressEncodeHash
{
    public static async Task<int> RunAsync(string[] args)
    {
        var text = string.Concat(Enumerable.Repeat("task tracker ", 500));
        var original = Encoding.UTF8.GetBytes(text);

        using var packed = new MemoryStream();
        await using (var gzip = new GZipStream(packed, CompressionLevel.Optimal, leaveOpen: true))
        {
            await gzip.WriteAsync(original);
        } // disposing the GZipStream writes the trailer; read `packed` only after this block

        packed.Position = 0;
        await using var unpacked = new GZipStream(packed, CompressionMode.Decompress);
        using var restored = new MemoryStream();
        await unpacked.CopyToAsync(restored);
        Console.WriteLine($"gzip smaller: {packed.Length < original.Length}, round trip equal: {restored.ToArray().AsSpan().SequenceEqual(original)}");

        using var zipBytes = new MemoryStream();
        using (var zip = new ZipArchive(zipBytes, ZipArchiveMode.Create, leaveOpen: true))
        {
            foreach (var name in new[] { "tasks.json", "notes/readme.txt" })
            {
                await using var entry = zip.CreateEntry(name).Open();
                await entry.WriteAsync("content of "u8.ToArray());
                await entry.WriteAsync(Encoding.UTF8.GetBytes(name));
            }
        }

        zipBytes.Position = 0;
        using (var zip = new ZipArchive(zipBytes, ZipArchiveMode.Read))
        {
            foreach (var entry in zip.Entries)
            {
                using var reader = new StreamReader(entry.Open());
                Console.WriteLine($"zip entry {entry.FullName}: {await reader.ReadToEndAsync()}");
            }
        }

        byte[] bytes = [0xFB, 0xFF, 0xFE, 0x00, 0x2A];
        Console.WriteLine($"hex        {Convert.ToHexStringLower(bytes)}");
        Console.WriteLine($"base64     {Convert.ToBase64String(bytes)}");
        Console.WriteLine($"base64url  {Base64Url.EncodeToString(bytes)}");
        Console.WriteLine($"decoded    {Convert.ToHexStringLower(Convert.FromBase64String("+//+ACo="))}");

        // Hash while streaming: never read the whole file into memory.
        using var stream = new MemoryStream("hello"u8.ToArray());
        var digest = await SHA256.HashDataAsync(stream);
        Console.WriteLine($"sha256     {Convert.ToHexStringLower(digest)}");

        var expected = SHA256.HashData("hello"u8);
        Console.WriteLine($"fixed-time compare: {CryptographicOperations.FixedTimeEquals(digest, expected)}");
        return 0;
    }
}
