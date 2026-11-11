using System.Text;

namespace M08.Examples;

/// <summary>Example 1: whole-file helpers, directories and encodings.</summary>
public static class ReadWriteFiles
{
    public static async Task<int> RunAsync(string[] args)
    {
        using var scratch = new ScratchDir();
        var notes = scratch.File("notes.txt");

        await File.WriteAllTextAsync(notes, "buy milk\nwrite lesson\n");
        await File.AppendAllTextAsync(notes, "ship it\n");
        var lines = await File.ReadAllLinesAsync(notes);
        Console.WriteLine($"{lines.Length} lines: {string.Join(" | ", lines)}");
        Console.WriteLine($"size on disk: {new FileInfo(notes).Length} bytes");

        // Characters are not bytes: the default encoding is UTF-8, so non-ASCII text takes more bytes.
        const string word = "café ☕";
        var utf8 = Encoding.UTF8.GetBytes(word);
        Console.WriteLine($"'{word}' = {word.Length} chars = {utf8.Length} UTF-8 bytes");

        var nested = Path.Combine(scratch.Path, "exports", "2030");
        Directory.CreateDirectory(nested); // creates every missing level, no error if it already exists
        File.Copy(notes, Path.Combine(nested, "notes-copy.txt"));
        File.Move(notes, Path.Combine(nested, "notes-moved.txt"));
        Console.WriteLine($"notes.txt exists after move: {File.Exists(notes)}");

        foreach (var file in Directory.EnumerateFiles(scratch.Path, "*.txt", SearchOption.AllDirectories).Order())
        {
            Console.WriteLine($"found {Path.GetRelativePath(scratch.Path, file).Replace('\\', '/')}");
        }

        File.Delete(Path.Combine(scratch.Path, "does-not-exist.txt")); // deleting a missing file is not an error
        try
        {
            await File.ReadAllTextAsync(Path.Combine(scratch.Path, "does-not-exist.txt"));
        }
        catch (FileNotFoundException)
        {
            Console.WriteLine("reading a missing file throws FileNotFoundException");
        }

        return 0;
    }
}
