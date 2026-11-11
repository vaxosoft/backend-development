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
    public static string Sanitize(string? clientName)
    {
        // Path.GetFileName only knows this OS's separators, but clients can send either style.
        var name = (clientName ?? "").Split('/', '\\')[^1];
        name = string.Concat(name.Where(c => !char.IsControl(c))).Trim();

        if (name.Length > MaxLength)
        {
            name = name[..MaxLength];
        }

        return name is "" or "." or ".." ? Fallback : name;
    }
}
