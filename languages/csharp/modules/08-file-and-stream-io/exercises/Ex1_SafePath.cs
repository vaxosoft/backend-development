namespace M08.Exercises;

public sealed class PathEscapeException(string userPath) : Exception($"'{userPath}' escapes the storage root");

public static class SafePath
{
    /// <summary>
    /// Maps an untrusted relative path onto a location under <paramref name="root"/>.
    /// <list type="bullet">
    /// <item>The user path is always treated as relative: a leading '/' or '\' does not make it absolute.</item>
    /// <item>Both '/' and '\' separate segments, on every operating system.</item>
    /// <item>".." segments are fine while the result stays inside the root (<c>a/../b</c>); otherwise throw
    /// <see cref="PathEscapeException"/>. A sibling such as <c>root-evil</c> is NOT inside <c>root</c>.</item>
    /// <item>A path containing a NUL character throws <see cref="PathEscapeException"/>.</item>
    /// <item>An empty path or "." resolves to the root itself. The result is a normalized full path.</item>
    /// </list>
    /// </summary>
    public static string Resolve(string root, string userPath)
    {
        throw new NotImplementedException();
    }

    /// <summary>
    /// Makes a client-supplied file name safe to show or store as metadata (it is never used as a path):
    /// keep only the part after the last '/' or '\', drop control characters, trim whitespace,
    /// cut to 100 characters, and return "unnamed" if nothing usable is left (empty, "." or "..").
    /// </summary>
    public static string SanitizeFileName(string clientName)
    {
        throw new NotImplementedException();
    }
}
