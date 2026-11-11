using System.IO.Compression;
using System.Reflection;
using System.Runtime.CompilerServices;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;

namespace M08.Examples;

/// <summary>Example 5: export tasks as gzipped JSON Lines with an atomic replace, then stream them back.</summary>
public static class TaskArchiveDemo
{
    public static async Task<int> RunAsync(string[] args)
    {
        await using var seedStream = Assembly.GetExecutingAssembly().GetManifestResourceStream("seed-tasks.json")
            ?? throw new InvalidOperationException("embedded resource missing");
        var seed = await JsonSerializer.DeserializeAsync<List<TaskItem>>(seedStream, Json.Options) ?? [];
        Console.WriteLine($"embedded seed holds {seed.Count} tasks");

        using var scratch = new ScratchDir();
        var archive = scratch.File("tasks.jsonl.gz");

        await TaskArchive.WriteAsync(archive, seed);
        await foreach (var task in TaskArchive.ReadAsync(archive))
        {
            Console.WriteLine($"  #{task.Id} {task.Title} [{task.Status}]");
        }

        Console.WriteLine($"sha256 of the JSON Lines content: {await TaskArchive.ContentHashAsync(archive)}");

        // A failure half-way through must leave the previous archive untouched and no temp file behind.
        try
        {
            await TaskArchive.WriteAsync(archive, FailsHalfway(seed));
        }
        catch (InvalidOperationException ex)
        {
            Console.WriteLine($"second export failed: {ex.Message}");
        }

        var survivors = await TaskArchive.ReadAsync(archive).CountAsync();
        Console.WriteLine($"archive still has {survivors} tasks; files in directory: {string.Join(", ", Directory.GetFiles(scratch.Path).Select(Path.GetFileName))}");
        return 0;
    }

    private static IEnumerable<TaskItem> FailsHalfway(IEnumerable<TaskItem> tasks)
    {
        foreach (var task in tasks.Take(2))
        {
            yield return task;
        }

        throw new InvalidOperationException("database went away");
    }
}

public static class TaskArchive
{
    public static async Task WriteAsync(string path, IEnumerable<TaskItem> tasks, CancellationToken cancellationToken = default)
    {
        // Same directory as the target so the final move is a rename, which is atomic on one volume.
        var temp = $"{path}.{Guid.NewGuid():N}.tmp";
        try
        {
            await using (var file = new FileStream(temp, FileMode.CreateNew, FileAccess.Write, FileShare.None, 4096, FileOptions.Asynchronous))
            await using (var gzip = new GZipStream(file, CompressionLevel.Optimal))
            await using (var writer = new StreamWriter(gzip, new UTF8Encoding(false)) { NewLine = "\n" })
            {
                foreach (var task in tasks)
                {
                    await writer.WriteLineAsync(JsonSerializer.Serialize(task, Json.Options).AsMemory(), cancellationToken);
                }
            }

            File.Move(temp, path, overwrite: true);
        }
        catch
        {
            File.Delete(temp);
            throw;
        }
    }

    public static async IAsyncEnumerable<TaskItem> ReadAsync(string path, [EnumeratorCancellation] CancellationToken cancellationToken = default)
    {
        await using var file = new FileStream(path, FileMode.Open, FileAccess.Read, FileShare.Read, 4096, FileOptions.Asynchronous | FileOptions.SequentialScan);
        await using var gzip = new GZipStream(file, CompressionMode.Decompress);
        using var reader = new StreamReader(gzip, Encoding.UTF8);
        while (await reader.ReadLineAsync(cancellationToken) is { Length: > 0 } line)
        {
            yield return JsonSerializer.Deserialize<TaskItem>(line, Json.Options)!;
        }
    }

    public static async Task<string> ContentHashAsync(string path, CancellationToken cancellationToken = default)
    {
        await using var file = File.OpenRead(path);
        await using var gzip = new GZipStream(file, CompressionMode.Decompress);
        return Convert.ToHexStringLower(await SHA256.HashDataAsync(gzip, cancellationToken));
    }
}
