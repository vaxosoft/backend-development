namespace M14.Exercises;

public class Ex2_ServerOptionsTests
{
    private static readonly IReadOnlyDictionary<string, string> NoEnv = new Dictionary<string, string>();

    private static IReadOnlyDictionary<string, string> Env(params (string Key, string Value)[] pairs) =>
        pairs.ToDictionary(p => p.Key, p => p.Value);

    [Fact]
    public void Defaults_WhenNothingIsSet()
    {
        var options = Ex2_ServerOptionsParser.Parse([], NoEnv);

        Assert.Equal(new ServerOptions("127.0.0.1", 8080, "Development", TimeSpan.FromSeconds(10)), options);
    }

    [Fact]
    public void EnvironmentVariablesOverrideDefaults()
    {
        var options = Ex2_ServerOptionsParser.Parse([], Env(
            ("ADDR", "0.0.0.0:9000"), ("ENVIRONMENT", "production"), ("SHUTDOWN_TIMEOUT_SECONDS", "30")));

        Assert.Equal(new ServerOptions("0.0.0.0", 9000, "Production", TimeSpan.FromSeconds(30)), options);
    }

    [Fact]
    public void FlagsOverrideEnvironmentVariables()
    {
        var options = Ex2_ServerOptionsParser.Parse(
            ["--addr", "127.0.0.1:7000", "--env=Staging", "--shutdown-timeout", "0"],
            Env(("ADDR", "0.0.0.0:9000"), ("ENVIRONMENT", "Production"), ("SHUTDOWN_TIMEOUT_SECONDS", "30")));

        Assert.Equal(new ServerOptions("127.0.0.1", 7000, "Staging", TimeSpan.Zero), options);
    }

    [Fact]
    public void PartialOverridesKeepTheOtherLayers()
    {
        var options = Ex2_ServerOptionsParser.Parse(["--env", "Staging"], Env(("ADDR", "0.0.0.0:9000")));

        Assert.Equal("0.0.0.0", options.Host);
        Assert.Equal(9000, options.Port);
        Assert.Equal("Staging", options.Environment);
    }

    [Fact]
    public void Ipv6HostsKeepTheirBrackets()
    {
        var options = Ex2_ServerOptionsParser.Parse(["--addr=[::1]:9000"], NoEnv);

        Assert.Equal("[::1]", options.Host);
        Assert.Equal(9000, options.Port);
    }

    [Theory]
    [InlineData("--addr", "localhost")]
    [InlineData("--addr", ":8080")]
    [InlineData("--addr", "host:abc")]
    [InlineData("--addr", "host:0")]
    [InlineData("--addr", "host:65536")]
    [InlineData("--env", "Testing")]
    [InlineData("--shutdown-timeout", "-1")]
    [InlineData("--shutdown-timeout", "301")]
    [InlineData("--shutdown-timeout", "soon")]
    public void InvalidFlagValues_NameTheFlag(string flag, string value)
    {
        var ex = Assert.Throws<FlagException>(() => Ex2_ServerOptionsParser.Parse([flag, value], NoEnv));

        Assert.Contains(flag, ex.Message);
    }

    [Fact]
    public void InvalidEnvironmentVariables_NameTheVariable()
    {
        var ex = Assert.Throws<FlagException>(() => Ex2_ServerOptionsParser.Parse([], Env(("ADDR", "nonsense"))));

        Assert.Contains("ADDR", ex.Message);
    }

    [Fact]
    public void UnknownFlagsAndMissingValuesAreErrors()
    {
        Assert.Contains("--bogus", Assert.Throws<FlagException>(() => Ex2_ServerOptionsParser.Parse(["--bogus", "1"], NoEnv)).Message);
        Assert.Contains("--addr", Assert.Throws<FlagException>(() => Ex2_ServerOptionsParser.Parse(["--addr"], NoEnv)).Message);
    }

    [Fact]
    public void AValidFlagWinsEvenWhenTheEnvironmentVariableIsInvalid()
    {
        var options = Ex2_ServerOptionsParser.Parse(["--addr", "127.0.0.1:7000"], Env(("ADDR", "nonsense")));

        Assert.Equal(7000, options.Port);
    }
}
