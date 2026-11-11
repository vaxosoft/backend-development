using System.Text;

namespace M08.Examples;

/// <summary>Example 2: Stream is the common abstraction; process data without loading it all.</summary>
public static class Streams
{
    public static async Task<int> RunAsync(string[] args)
    {
        using var scratch = new ScratchDir();
        var path = scratch.File("big.log");

        // await using disposes (and therefore flushes) the writer and the FileStream, in reverse order.
        await using (var file = new FileStream(path, new FileStreamOptions
        {
            Mode = FileMode.Create,
            Access = FileAccess.Write,
            Share = FileShare.Read,
            Options = FileOptions.Asynchronous,
        }))
        await using (var writer = new StreamWriter(file, new UTF8Encoding(false)))
        {
            for (var i = 1; i <= 100_000; i++)
            {
                await writer.WriteLineAsync($"line {i}");
            }
        }

        Console.WriteLine($"wrote {new FileInfo(path).Length} bytes");

        // Constant memory: one line at a time, however large the file is.
        var lines = 0;
        string? last = null;
        using (var reader = new StreamReader(path))
        {
            while (await reader.ReadLineAsync() is { } line)
            {
                lines++;
                last = line;
            }
        }

        Console.WriteLine($"read {lines} lines, last = '{last}'");

        // The same method works for any Stream: file, memory, network, decompressor...
        await using var fromFile = File.OpenRead(path);
        using var fromMemory = new MemoryStream("hello streams"u8.ToArray());
        Console.WriteLine($"CountBytes(file)   = {await CountBytesAsync(fromFile)}");
        Console.WriteLine($"CountBytes(memory) = {await CountBytesAsync(fromMemory)}");

        // Buffered data is invisible to other readers until it is flushed.
        var live = scratch.File("live.txt");
        await using var liveFile = new FileStream(live, FileMode.Create, FileAccess.Write, FileShare.ReadWrite);
        await using var liveWriter = new StreamWriter(liveFile);
        await liveWriter.WriteAsync("partial");
        Console.WriteLine($"before flush: {new FileInfo(live).Length} bytes on disk");
        await liveWriter.FlushAsync();
        Console.WriteLine($"after flush:  {new FileInfo(live).Length} bytes on disk");

        return 0;
    }

    public static async Task<long> CountBytesAsync(Stream stream, CancellationToken cancellationToken = default)
    {
        var buffer = new byte[8192];
        long total = 0;
        int read;
        while ((read = await stream.ReadAsync(buffer, cancellationToken)) > 0)
        {
            total += read;
        }

        return total;
    }
}
