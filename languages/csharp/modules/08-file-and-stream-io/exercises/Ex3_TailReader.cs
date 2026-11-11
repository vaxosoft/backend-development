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
    public static Task<IReadOnlyList<string>> LastLinesAsync(Stream stream, int count, CancellationToken cancellationToken = default)
    {
        throw new NotImplementedException();
    }
}
