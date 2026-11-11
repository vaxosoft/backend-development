namespace M08.Examples;

/// <summary>Example 3: Path.Combine surprises and keeping user-supplied names inside a root directory.</summary>
public static class PathsAndSafety
{
    public static async Task<int> RunAsync(string[] args)
    {
        using var scratch = new ScratchDir();
        var root = Path.Combine(scratch.Path, "uploads");
        Directory.CreateDirectory(root);

        Console.WriteLine("-- Path.Combine vs Path.Join (second argument is rooted)");
        Console.WriteLine($"Combine(root, \"/etc/passwd\") is inside root: {IsInside(root, Path.Combine(root, "/etc/passwd"))}");
        Console.WriteLine($"Join(root, \"/etc/passwd\")    is inside root: {IsInside(root, Path.Join(root, "/etc/passwd"))}");

        Console.WriteLine("-- traversal check");
        foreach (var name in new[] { "report.txt", "sub/report.txt", "../secret.txt", "a/../../secret.txt", "/etc/passwd", "../uploads-evil/x" })
        {
            Console.WriteLine($"{name,-22} -> {(TryResolve(root, name, out _) ? "allowed" : "rejected")}");
        }

        Console.WriteLine("-- storing under a generated id, the client name is only metadata");
        var clientName = "../../etc/passwd";
        var storedName = SanitizeFileName(clientName);
        var target = Path.Combine(root, Guid.CreateVersion7().ToString("N"));
        await File.WriteAllTextAsync(target, "uploaded bytes");
        Console.WriteLine($"client name '{clientName}' -> display name '{storedName}'");
        Console.WriteLine($"stored in root: {IsInside(root, target)}, file count in root: {Directory.GetFiles(root).Length}");
        return 0;
    }

    /// <summary>Resolves <paramref name="userPath"/> under <paramref name="root"/>; false if it would escape.</summary>
    public static bool TryResolve(string root, string userPath, out string fullPath)
    {
        // Treat the user path as relative even when it starts with '/'.
        fullPath = Path.GetFullPath(Path.Join(root, userPath));
        return IsInside(root, fullPath);
    }

    public static bool IsInside(string root, string candidate)
    {
        var rootFull = Path.TrimEndingDirectorySeparator(Path.GetFullPath(root)) + Path.DirectorySeparatorChar;
        // The trailing separator matters: "/data-evil/x" starts with "/data" but is not inside it.
        return Path.GetFullPath(candidate).StartsWith(rootFull, StringComparison.Ordinal);
    }

    public static string SanitizeFileName(string clientName)
    {
        var name = clientName.Replace('\\', '/');
        name = name[(name.LastIndexOf('/') + 1)..];
        name = string.Concat(name.Where(c => !char.IsControl(c)));
        return name is "" or "." or ".." ? "unnamed" : name;
    }
}
