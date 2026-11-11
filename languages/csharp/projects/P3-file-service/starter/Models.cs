using System.Text.Json;

namespace P3.FileService;

/// <summary>Bound from the <c>FileService</c> configuration section (env: <c>FileService__StorageDir</c>, ...).</summary>
public sealed class FileServiceOptions
{
    public const string SectionName = "FileService";

    /// <summary>Directory that holds every stored file. Created on startup.</summary>
    public string StorageDir { get; set; } = "data";

    /// <summary>Largest accepted file, in bytes (default 5 MiB).</summary>
    public long MaxUploadBytes { get; set; } = 5 * 1024 * 1024;
}

/// <summary>Metadata of a stored file. This is also the JSON body returned by the upload.</summary>
public sealed record StoredFile(string Id, string Name, long Size, string Sha256, string ContentType);

/// <summary>A stored file opened for reading. The caller owns <see cref="Content"/>.</summary>
public sealed record StoredDownload(StoredFile File, FileStream Content);

public sealed class FileTooLargeException(long limit) : Exception($"file exceeds the limit of {limit} bytes");

public static class StoredJson
{
    public static readonly JsonSerializerOptions Options = new(JsonSerializerDefaults.Web)
    {
        PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower,
    };
}
