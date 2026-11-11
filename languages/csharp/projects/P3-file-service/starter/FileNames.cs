namespace P3.FileService;

public static class FileNames
{
    public const string Fallback = "file";
    public const int MaxLength = 255;

    /// <summary>
    /// Turns a client-supplied file name into a harmless display name: no directory parts
    /// (both <c>/</c> and <c>\</c> count as separators), no control characters, never empty,
    /// never <c>.</c> or <c>..</c>, at most <see cref="MaxLength"/> characters.
    /// </summary>
    /// <remarks>The result is only ever shown to people; it is never used to build a path.</remarks>
    public static string Sanitize(string? clientName) =>
        throw new NotImplementedException("TODO: keep the last path segment, drop control characters, apply the fallback");
}
