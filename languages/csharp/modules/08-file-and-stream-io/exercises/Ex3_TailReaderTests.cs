using System.Text;

namespace M08.Exercises;

public class Ex3_TailReaderTests
{
    private static MemoryStream Utf8(string text) => new(Encoding.UTF8.GetBytes(text));

    [Theory]
    [InlineData("a\nb\nc\n", 2, "b|c")]
    [InlineData("a\nb\nc", 2, "b|c")]
    [InlineData("a\r\nb\r\nc\r\n", 2, "b|c")]
    [InlineData("a\nb\nc\n", 10, "a|b|c")]
    [InlineData("a\n\nc\n", 2, "|c")]
    [InlineData("only", 1, "only")]
    [InlineData("a\nb\n", 0, "")]
    [InlineData("", 3, "")]
    public async Task Returns_the_last_lines_oldest_first(string content, int count, string expected)
    {
        var lines = await TailReader.LastLinesAsync(Utf8(content), count);

        Assert.Equal(expected.Length == 0 ? [] : expected.Split('|'), lines);
    }

    [Fact]
    public async Task Reads_only_the_end_of_a_large_stream()
    {
        var text = new StringBuilder();
        for (var i = 1; i <= 200_000; i++)
        {
            text.Append("line ").Append(i).Append('\n');
        }

        var counting = new CountingStream(Utf8(text.ToString()));

        var lines = await TailReader.LastLinesAsync(counting, 3);

        Assert.Equal(["line 199998", "line 199999", "line 200000"], lines);
        Assert.True(counting.BytesRead < counting.Length / 20, $"read {counting.BytesRead} of {counting.Length} bytes");
    }

    [Fact]
    public async Task Lines_longer_than_a_chunk_and_multibyte_text_survive()
    {
        var longLine = new string('é', 20_000) + "☕";
        var content = "first\n" + longLine + "\nlast ☕\n";

        var lines = await TailReader.LastLinesAsync(Utf8(content), 2);

        Assert.Equal([longLine, "last ☕"], lines);
    }

    [Fact]
    public async Task The_stream_position_does_not_matter()
    {
        var stream = Utf8("a\nb\nc\n");
        stream.Position = 3;

        Assert.Equal(["b", "c"], await TailReader.LastLinesAsync(stream, 2));
    }

    [Fact]
    public async Task Works_on_real_files()
    {
        using var dir = new TempDir();
        var path = Path.Combine(dir.Path, "app.log");
        await File.WriteAllLinesAsync(path, Enumerable.Range(1, 1000).Select(i => $"entry {i}"));

        await using var file = new FileStream(path, FileMode.Open, FileAccess.Read, FileShare.Read, 4096, FileOptions.Asynchronous);

        Assert.Equal(["entry 999", "entry 1000"], await TailReader.LastLinesAsync(file, 2));
    }
}
