namespace M14.Exercises;

public class Ex1_BuildInfoTests
{
    [Theory]
    [InlineData("1.4.2", "1.4.2", null, null)]
    [InlineData("1.4.2+abc1234", "1.4.2", null, "abc1234")]
    [InlineData("1.4.2+abc1234567890", "1.4.2", null, "abc1234")]
    [InlineData("1.4.2-rc.1", "1.4.2", "rc.1", null)]
    [InlineData("1.4.2-rc.1+abc1234def", "1.4.2", "rc.1", "abc1234")]
    [InlineData("10.0.0+ab", "10.0.0", null, "ab")]
    [InlineData("1.4.2+", "1.4.2", null, null)]
    public void Parse_SplitsVersionPreReleaseAndCommit(string input, string version, string? pre, string? commit)
    {
        var info = Ex1_BuildInfo.Parse(input);

        Assert.Equal(version, info.Version);
        Assert.Equal(pre, info.PreRelease);
        Assert.Equal(commit, info.Commit);
        Assert.Equal(pre is not null, info.IsPreRelease);
    }

    [Theory]
    [InlineData("")]
    [InlineData("1.4")]
    [InlineData("1.4.2.7")]
    [InlineData("a.b.c")]
    [InlineData("1.4.-2")]
    [InlineData("v1.4.2")]
    [InlineData("1.4.2-")]
    [InlineData("+abc1234")]
    public void Parse_RejectsMalformedVersions(string input) =>
        Assert.Throws<FormatException>(() => Ex1_BuildInfo.Parse(input));

    [Theory]
    [InlineData("1.4.2", "1.4.2")]
    [InlineData("1.4.2+abc1234def", "1.4.2 (abc1234)")]
    [InlineData("1.4.2-rc.1", "1.4.2-rc.1")]
    [InlineData("1.4.2-rc.1+abc1234def", "1.4.2-rc.1 (abc1234)")]
    public void ToDisplayString_IsHumanReadable(string input, string expected) =>
        Assert.Equal(expected, Ex1_BuildInfo.Parse(input).ToDisplayString());
}
