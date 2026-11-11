namespace M13.Exercises;

public class Ex2_TextTests
{
    [Theory]
    [InlineData("Hello, World!", "hello-world")]
    [InlineData("  spaced   out  ", "spaced-out")]
    [InlineData("Write docs #p1", "write-docs-p1")]
    [InlineData("--already-slug--", "already-slug")]
    [InlineData("Q3 2030 Report", "q3-2030-report")]
    [InlineData("!!!", "")]
    [InlineData("", "")]
    public void Slugify(string input, string expected) => Assert.Equal(expected, Ex2_Text.Slugify(input));

    [Fact]
    public void Slugify_IsFastOnLongSeparatorRuns()
    {
        var input = new string('-', 50_000) + "x" + new string('!', 50_000);

        Assert.Equal("x", Ex2_Text.Slugify(input));
    }

    [Fact]
    public void ExtractTags_LowercasesDeduplicatesAndKeepsOrder() =>
        Assert.Equal(["docs", "p1", "q3-plan"], Ex2_Text.ExtractTags("Fix #Docs and #p1, then #docs again #Q3-plan."));

    [Fact]
    public void ExtractTags_IgnoresOverlongAndEmptyTags()
    {
        var overlong = "#" + new string('a', 31);

        Assert.Empty(Ex2_Text.ExtractTags($"{overlong} # #!"));
        Assert.Equal([new string('b', 30)], Ex2_Text.ExtractTags($"#{new string('b', 30)}"));
    }

    [Fact]
    public void Render_ReplacesPlaceholdersAndEncodesValues()
    {
        var values = new Dictionary<string, string> { ["name"] = "<Ada & Bob>", ["count"] = "3" };

        Assert.Equal(
            "Hi &lt;Ada &amp; Bob&gt;, 3 tasks (3).",
            Ex2_Text.Render("Hi {{name}}, {{ count }} tasks ({{count}}).", values));
    }

    [Fact]
    public void Render_LeavesTextWithoutPlaceholdersAlone() =>
        Assert.Equal("a { b } {c}", Ex2_Text.Render("a { b } {c}", new Dictionary<string, string>()));

    [Fact]
    public void Render_ThrowsForUnknownPlaceholders()
    {
        var ex = Assert.Throws<KeyNotFoundException>(() => Ex2_Text.Render("Hi {{who}}", new Dictionary<string, string>()));

        Assert.Contains("who", ex.Message);
    }

    [Fact]
    public void Render_DoesNotReprocessSubstitutedValues()
    {
        var values = new Dictionary<string, string> { ["a"] = "{{b}}", ["b"] = "boom" };

        Assert.Equal("{{b}}", Ex2_Text.Render("{{a}}", values));
    }
}
