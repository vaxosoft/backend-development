using System.Security.Cryptography;

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
    public async Task<StoredFile> SaveAsync(Stream input, CancellationToken cancellationToken = default)
    {
        Directory.CreateDirectory(directory);
        var id = Guid.NewGuid().ToString("N");
        var path = Path.Combine(directory, id);
        try
        {
            using var hash = IncrementalHash.CreateHash(HashAlgorithmName.SHA256);
            long size = 0;
            await using (var file = new FileStream(path, FileMode.CreateNew, FileAccess.Write, FileShare.None, 4096, FileOptions.Asynchronous))
            {
                var buffer = new byte[8192];
                int read;
                while ((read = await input.ReadAsync(buffer, cancellationToken)) > 0)
                {
                    size += read;
                    if (size > maxBytes)
                    {
                        throw new FileTooLargeException(maxBytes);
                    }

                    hash.AppendData(buffer, 0, read);
                    await file.WriteAsync(buffer.AsMemory(0, read), cancellationToken);
                }
            }

            return new StoredFile(id, size, Convert.ToHexStringLower(hash.GetHashAndReset()));
        }
        catch
        {
            File.Delete(path);
            throw;
        }
    }

    /// <summary>
    /// Opens a stored file for reading, or returns null if the id is unknown. Anything that is not a
    /// 32-character lowercase hex id (for example <c>../x</c>) is unknown, never a path.
    /// </summary>
    public Stream? Open(string id)
    {
        if (id.Length != 32 || !id.All(c => c is (>= '0' and <= '9') or (>= 'a' and <= 'f')))
        {
            return null;
        }

        var path = Path.Combine(directory, id);
        return File.Exists(path) ? File.OpenRead(path) : null;
    }
}
