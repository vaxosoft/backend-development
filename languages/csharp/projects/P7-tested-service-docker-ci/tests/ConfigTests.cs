using Microsoft.Extensions.Logging;

namespace P7.TestedService.Tests;

public class ConfigTests
{
    private static ConfigResult Parse(params (string Key, string Value)[] env)
    {
        var map = env.ToDictionary(e => e.Key, e => e.Value);
        return AppConfig.Parse(key => map.GetValueOrDefault(key));
    }

    [Fact]
    public void Defaults_apply_when_nothing_is_set()
    {
        var result = Parse();

        Assert.True(result.IsValid);
        Assert.Equal(new AppConfig(8080, LogLevel.Information, "dev"), result.Config);
    }

    [Fact]
    public void Blank_values_count_as_unset()
    {
        var result = Parse(("PORT", ""), ("LOG_LEVEL", "  "), ("ENV", ""));

        Assert.True(result.IsValid);
        Assert.Equal(new AppConfig(8080, LogLevel.Information, "dev"), result.Config);
    }

    [Fact]
    public void Valid_values_are_read_case_insensitively()
    {
        var result = Parse(("PORT", "9090"), ("LOG_LEVEL", "WARN"), ("ENV", "Prod"));

        Assert.True(result.IsValid);
        Assert.Equal(new AppConfig(9090, LogLevel.Warning, "prod"), result.Config);
    }

    [Theory]
    [InlineData("debug", LogLevel.Debug)]
    [InlineData("info", LogLevel.Information)]
    [InlineData("warn", LogLevel.Warning)]
    [InlineData("error", LogLevel.Error)]
    public void Every_documented_log_level_is_accepted(string text, LogLevel expected)
    {
        Assert.Equal(expected, Parse(("LOG_LEVEL", text)).Config?.LogLevel);
    }

    [Theory]
    [InlineData("abc")]
    [InlineData("0")]
    [InlineData("65536")]
    [InlineData("-1")]
    [InlineData("80.5")]
    public void Invalid_port_is_reported_with_the_variable_name_and_value(string port)
    {
        var result = Parse(("PORT", port));

        Assert.False(result.IsValid);
        Assert.Null(result.Config);
        var error = Assert.Single(result.Errors);
        Assert.Contains("PORT", error);
        Assert.Contains(port, error);
    }

    [Theory]
    [InlineData("trace", "LOG_LEVEL")]
    [InlineData("verbose", "LOG_LEVEL")]
    [InlineData("production", "ENV")]
    public void Unknown_log_level_or_environment_is_reported(string value, string key)
    {
        var result = Parse((key, value));

        Assert.False(result.IsValid);
        Assert.Contains(key, Assert.Single(result.Errors));
    }

    [Fact]
    public void All_problems_are_reported_at_once()
    {
        var result = Parse(("PORT", "x"), ("LOG_LEVEL", "loud"), ("ENV", "moon"));

        Assert.Equal(3, result.Errors.Count);
    }
}
