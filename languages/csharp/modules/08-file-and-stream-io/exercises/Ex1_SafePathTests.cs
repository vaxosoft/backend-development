namespace M08.Exercises;

public class Ex1_SafePathTests
{
    private static readonly string Root = Path.GetFullPath(Path.Combine(Path.GetTempPath(), "m08-root", "data"));

    [Theory]
    [InlineData("a.txt", "a.txt")]
    [InlineData("sub/a.txt", "sub/a.txt")]
    [InlineData("sub\\a.txt", "sub/a.txt")]
    [InlineData("a/../b.txt", "b.txt")]
    [InlineData("./a.txt", "a.txt")]
    [InlineData("/a.txt", "a.txt")]
    [InlineData("\\a.txt", "a.txt")]
    public void Paths_that_stay_inside_the_root_resolve_below_it(string userPath, string expectedRelative)
    {
        var expected = Path.GetFullPath(Path.Combine(Root, expectedRelative));

        Assert.Equal(expected, SafePath.Resolve(Root, userPath));
    }

    [Theory]
    [InlineData("")]
    [InlineData(".")]
    public void The_root_itself_is_allowed(string userPath)
    {
        Assert.Equal(Root, SafePath.Resolve(Root, userPath));
    }

    [Theory]
    [InlineData("../secret.txt")]
    [InlineData("a/../../secret.txt")]
    [InlineData("..\\secret.txt")]
    [InlineData("/../secret.txt")]
    [InlineData("../data-evil/x.txt")]
    [InlineData("..")]
    [InlineData("a\0b")]
    public void Paths_that_escape_the_root_are_rejected(string userPath)
    {
        Assert.Throws<PathEscapeException>(() => SafePath.Resolve(Root, userPath));
    }

    [Theory]
    [InlineData("report.pdf", "report.pdf")]
    [InlineData("../../etc/passwd", "passwd")]
    [InlineData("C:\\Users\\ada\\cv.docx", "cv.docx")]
    [InlineData("  spaced name.txt  ", "spaced name.txt")]
    [InlineData("bad\r\nname\t.txt", "badname.txt")]
    [InlineData("..", "unnamed")]
    [InlineData(".", "unnamed")]
    [InlineData("", "unnamed")]
    [InlineData("dir/", "unnamed")]
    public void Client_file_names_are_reduced_to_a_harmless_display_name(string clientName, string expected)
    {
        Assert.Equal(expected, SafePath.SanitizeFileName(clientName));
    }

    [Fact]
    public void Very_long_names_are_cut()
    {
        Assert.Equal(100, SafePath.SanitizeFileName(new string('a', 500) + ".txt").Length);
    }
}
