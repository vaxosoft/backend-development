namespace M10.Exercises;

public class Ex1_LogSafetyTests
{
    private const string Generated = "generated-id";

    private static string Resolve(string? incoming) => LogSafety.ResolveCorrelationId(incoming, () => Generated);

    [Theory]
    [InlineData("abc-123", "abc-123")]
    [InlineData("  req_42.v2  ", "req_42.v2")]
    [InlineData("A", "A")]
    public void A_clean_incoming_id_is_kept(string incoming, string expected)
    {
        Assert.Equal(expected, Resolve(incoming));
    }

    [Theory]
    [InlineData(null)]
    [InlineData("")]
    [InlineData("   ")]
    [InlineData("has space")]
    [InlineData("line1\nINFO fake log line")]
    [InlineData("tab\there")]
    [InlineData("semi;colon")]
    [InlineData("unicode-é")]
    public void A_missing_or_suspicious_id_is_replaced(string? incoming)
    {
        Assert.Equal(Generated, Resolve(incoming));
    }

    [Fact]
    public void Length_is_capped_at_64()
    {
        Assert.Equal(new string('a', 64), Resolve(new string('a', 64)));
        Assert.Equal(Generated, Resolve(new string('a', 65)));
    }

    [Fact]
    public void The_generator_is_only_called_when_needed()
    {
        var calls = 0;
        LogSafety.ResolveCorrelationId("ok", () => { calls++; return "x"; });
        LogSafety.ResolveCorrelationId(null, () => { calls++; return "x"; });

        Assert.Equal(1, calls);
    }

    [Theory]
    [InlineData(null, "")]
    [InlineData("", "")]
    [InlineData("page=2&sort=title", "page=2&sort=title")]
    [InlineData("?page=2&token=abc123", "?page=2&token=***")]
    [InlineData("password=hunter2&user=ada", "password=***&user=ada")]
    [InlineData("Access_Token=xyz&API_KEY=k&ApiKey=k2", "Access_Token=***&API_KEY=***&ApiKey=***")]
    [InlineData("secret=a%20b&flag&authorization=Bearer%20abc", "secret=***&flag&authorization=***")]
    [InlineData("tokens=ok&mytoken=ok", "tokens=ok&mytoken=ok")]
    [InlineData("token=&q=1", "token=***&q=1")]
    public void Sensitive_query_values_are_masked(string? query, string expected)
    {
        Assert.Equal(expected, LogSafety.RedactQuery(query));
    }
}
