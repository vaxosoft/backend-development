using Microsoft.Extensions.Options;

namespace P3.FileService;

/// <summary>
/// Files live in <c>{root}/{id}.bin</c> with their metadata in <c>{root}/{id}.json</c>.
/// Ids are generated (32 hex characters); client names never reach the file system.
/// </summary>
public sealed class FileStore
{
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
    private string PathFor(string id, string extension) =>
        throw new NotImplementedException("TODO: validate the id, combine, and verify the full path is inside _root");

    /// <summary>
    /// Streams <paramref name="content"/> to disk while hashing it, never holding the whole file in memory.
    /// Nothing may remain on disk when this throws (too large, cancelled, I/O error).
    /// </summary>
    /// <exception cref="FileTooLargeException">More than <see cref="MaxUploadBytes"/> bytes were sent.</exception>
    public Task<StoredFile> SaveAsync(Stream content, string? clientName, string? contentType, CancellationToken ct) =>
        throw new NotImplementedException("TODO: copy in chunks, count bytes, SHA-256 incrementally, clean up on failure");

    /// <summary>Opens a stored file, or returns <c>null</c> for unknown or malformed ids.</summary>
    public StoredDownload? Open(string id) =>
        throw new NotImplementedException("TODO: read the metadata, open the content stream read-only");

    /// <returns><c>true</c> when the file existed.</returns>
    public bool Delete(string id) =>
        throw new NotImplementedException("TODO: remove content and metadata");
}
