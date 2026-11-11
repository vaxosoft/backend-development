#pragma warning disable CS9113 // starter: the constructor parameters are for your implementation to use

namespace M08.Exercises;

public sealed record StoredFile(string Id, long Size, string Sha256);

public sealed class FileTooLargeException(long limit) : Exception($"file exceeds {limit} bytes");

/// <summary>
/// Stores uploads under generated ids in <paramref name="directory"/> (created on demand).
/// </summary>
public sealed class UploadStore(string directory, long maxBytes)
{
    /// <summary>
    /// Streams <paramref name="input"/> to a new file named by a fresh id (32 lowercase hex characters).
    /// Returns the id, the size in bytes and the lowercase hex SHA-256 of the content.
    /// <list type="bullet">
    /// <item>Never buffer the whole input: copy in small chunks and hash incrementally.</item>
    /// <item>More than <c>maxBytes</c> bytes (exactly <c>maxBytes</c> is fine): stop reading, delete the partial file
    /// and throw <see cref="FileTooLargeException"/>. Do not keep pulling from an endless input.</item>
    /// <item>Any failure (including cancellation) must leave no partial file behind.</item>
    /// </list>
    /// </summary>
    public Task<StoredFile> SaveAsync(Stream input, CancellationToken cancellationToken = default)
    {
        throw new NotImplementedException();
    }

    /// <summary>
    /// Opens a stored file for reading, or returns null if the id is unknown. Anything that is not a
    /// 32-character lowercase hex id (for example <c>../x</c>) is unknown, never a path.
    /// </summary>
    public Stream? Open(string id)
    {
        throw new NotImplementedException();
    }
}
