using System.Security.Cryptography;
using System.Text.Json;
using Microsoft.Extensions.Options;
using Microsoft.Net.Http.Headers;

namespace P3.FileService;

/// <summary>
/// Files live in <c>{root}/{id}.bin</c> with their metadata in <c>{root}/{id}.json</c>.
/// Ids are generated (32 hex characters); client names never reach the file system.
/// </summary>
public sealed class FileStore
{
    private const int BufferSize = 80 * 1024;

    private readonly string _root;

    public FileStore(IOptions<FileServiceOptions> options)
    {
        MaxUploadBytes = options.Value.MaxUploadBytes;
        _root = Path.TrimEndingDirectorySeparator(Path.GetFullPath(options.Value.StorageDir));
        Directory.CreateDirectory(_root);
    }

    public long MaxUploadBytes { get; }

    /// <summary>True for the ids this store hands out (32 lowercase hex characters).</summary>
    public static bool IsValidId(string? id) =>
        id is { Length: 32 } && id.All(c => c is (>= '0' and <= '9') or (>= 'a' and <= 'f'));

    /// <summary>Path of a file belonging to <paramref name="id"/>; must never leave the storage directory.</summary>
    private string PathFor(string id, string extension)
    {
        if (!IsValidId(id))
        {
            throw new ArgumentException("not a valid file id", nameof(id));
        }

        var path = Path.GetFullPath(Path.Combine(_root, id + extension));
        if (Path.GetDirectoryName(path) != _root)
        {
            throw new InvalidOperationException("path escapes the storage directory");
        }

        return path;
    }

    /// <summary>
    /// Streams <paramref name="content"/> to disk while hashing it, never holding the whole file in memory.
    /// Nothing may remain on disk when this throws (too large, cancelled, I/O error).
    /// </summary>
    /// <exception cref="FileTooLargeException">More than <see cref="MaxUploadBytes"/> bytes were sent.</exception>
    public async Task<StoredFile> SaveAsync(Stream content, string? clientName, string? contentType, CancellationToken ct)
    {
        var id = Guid.NewGuid().ToString("N");
        var partPath = PathFor(id, ".part");
        var contentPath = PathFor(id, ".bin");
        var metadataPath = PathFor(id, ".json");

        try
        {
            using var hash = IncrementalHash.CreateHash(HashAlgorithmName.SHA256);
            long size = 0;

            await using (var output = new FileStream(
                partPath, FileMode.CreateNew, FileAccess.Write, FileShare.None, BufferSize, FileOptions.Asynchronous))
            {
                var buffer = new byte[BufferSize];
                int read;
                while ((read = await content.ReadAsync(buffer, ct)) > 0)
                {
                    size += read;
                    if (size > MaxUploadBytes)
                    {
                        throw new FileTooLargeException(MaxUploadBytes);
                    }

                    hash.AppendData(buffer, 0, read);
                    await output.WriteAsync(buffer.AsMemory(0, read), ct);
                }
            }

            File.Move(partPath, contentPath);
            var stored = new StoredFile(id, FileNames.Sanitize(clientName), size, Convert.ToHexStringLower(hash.GetHashAndReset()), NormalizeContentType(contentType));

            // The metadata file is written last: a file only exists for readers once it is complete.
            await File.WriteAllTextAsync(metadataPath, JsonSerializer.Serialize(stored, StoredJson.Options), ct);
            return stored;
        }
        catch
        {
            File.Delete(partPath);
            File.Delete(contentPath);
            File.Delete(metadataPath);
            throw;
        }
    }

    /// <summary>Opens a stored file, or returns <c>null</c> for unknown or malformed ids.</summary>
    public StoredDownload? Open(string id)
    {
        if (!IsValidId(id))
        {
            return null;
        }

        try
        {
            var stored = JsonSerializer.Deserialize<StoredFile>(File.ReadAllText(PathFor(id, ".json")), StoredJson.Options);
            var content = new FileStream(PathFor(id, ".bin"), FileMode.Open, FileAccess.Read, FileShare.Read, BufferSize, FileOptions.Asynchronous | FileOptions.SequentialScan);
            return stored is null ? null : new StoredDownload(stored, content);
        }
        catch (FileNotFoundException)
        {
            return null;
        }
    }

    /// <returns><c>true</c> when the file existed.</returns>
    public bool Delete(string id)
    {
        if (!IsValidId(id) || !File.Exists(PathFor(id, ".json")))
        {
            return false;
        }

        File.Delete(PathFor(id, ".json"));
        File.Delete(PathFor(id, ".bin"));
        return true;
    }

    private static string NormalizeContentType(string? contentType) =>
        MediaTypeHeaderValue.TryParse(contentType, out var parsed) && parsed.MediaType.HasValue
            ? parsed.MediaType.Value!
            : "application/octet-stream";
}
