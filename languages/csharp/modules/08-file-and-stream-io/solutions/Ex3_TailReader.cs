using System.Text;

namespace M08.Exercises;

public static class TailReader
{
    /// <summary>
    /// Returns the last <paramref name="count"/> lines of a seekable UTF-8 stream, oldest first, without
    /// reading the whole stream: seek backwards and read chunks from the end until enough lines are found.
    /// <list type="bullet">
    /// <item>Lines end with '\n'; a trailing '\r' is removed. A final newline does not create an extra empty line.</item>
    /// <item>A last line without a newline still counts. Empty lines in the middle count as lines.</item>
    /// <item>If the stream has fewer than <paramref name="count"/> lines return all of them; count &lt;= 0 returns none.</item>
    /// <item>Lines may be longer than one chunk and may contain multi-byte characters.</item>
    /// </list>
    /// </summary>
    public static async Task<IReadOnlyList<string>> LastLinesAsync(Stream stream, int count, CancellationToken cancellationToken = default)
    {
        const int ChunkSize = 4096;
        if (count <= 0 || stream.Length == 0)
        {
            return [];
        }

        // Newlines found so far, counted from the end; we need count + 1 separators (or the start of the stream)
        // unless the stream ends with a newline, which does not start a new line.
        var tail = new List<byte[]>();
        var end = stream.Length;
        var start = end;
        var newlines = 0;
        var skippedFinalNewline = false;

        while (start > 0 && newlines <= count)
        {
            var size = (int)Math.Min(ChunkSize, start);
            start -= size;
            stream.Seek(start, SeekOrigin.Begin);
            var chunk = new byte[size];
            await stream.ReadExactlyAsync(chunk, cancellationToken);
            tail.Add(chunk);

            for (var i = size - 1; i >= 0; i--)
            {
                if (chunk[i] != (byte)'\n')
                {
                    continue;
                }

                if (!skippedFinalNewline && start + i == end - 1)
                {
                    skippedFinalNewline = true;
                    continue;
                }

                newlines++;
            }
        }

        tail.Reverse();
        var all = new byte[end - start];
        var offset = 0;
        foreach (var chunk in tail)
        {
            chunk.CopyTo(all, offset);
            offset += chunk.Length;
        }

        var text = Encoding.UTF8.GetString(all);
        if (text.EndsWith('\n'))
        {
            text = text[..^1];
        }

        var lines = text.Split('\n').Select(l => l.TrimEnd('\r')).ToList();
        return lines.Count > count ? lines[^count..] : lines;
    }
}
