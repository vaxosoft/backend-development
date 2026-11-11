namespace M09.Exercises;

public class Ex1_LayeredConfigTests
{
    private const string BaseJson = """
        { "Server": { "Host": "127.0.0.1", "Port": 8080 }, "Logging": { "Level": "Information" }, "Db": { "Password": "hunter2", "ApiKey": "abc", "Name": "tasks" } }
        """;

    private const string DevJson = """{ "Server": { "Port": 5000 }, "Logging": { "Level": "Debug" } }""";

    private static string UniquePrefix(string test) => $"M09EX1{test}{Guid.NewGuid():N}_";

    [Fact]
    public void Later_json_overrides_earlier_json_and_missing_files_are_skipped()
    {
        var config = LayeredConfig.Build(BaseJson, DevJson, UniquePrefix("a"), []);

        Assert.Equal("5000", config["Server:Port"]);
        Assert.Equal("127.0.0.1", config["Server:Host"]);
        Assert.Equal("Debug", config["Logging:Level"]);

        var onlyBase = LayeredConfig.Build(BaseJson, null, UniquePrefix("b"), []);
        Assert.Equal("8080", onlyBase["Server:Port"]);

        Assert.Null(LayeredConfig.Build(null, null, UniquePrefix("c"), [])["Server:Port"]);
    }

    [Fact]
    public void Environment_variables_with_the_prefix_beat_json_and_use_double_underscore_for_nesting()
    {
        var prefix = UniquePrefix("env");
        Environment.SetEnvironmentVariable(prefix + "Server__Port", "9090");
        Environment.SetEnvironmentVariable("OTHER_" + prefix + "Server__Host", "ignored.example");
        try
        {
            var config = LayeredConfig.Build(BaseJson, DevJson, prefix, []);

            Assert.Equal("9090", config["Server:Port"]);
            Assert.Equal("127.0.0.1", config["Server:Host"]);
        }
        finally
        {
            Environment.SetEnvironmentVariable(prefix + "Server__Port", null);
            Environment.SetEnvironmentVariable("OTHER_" + prefix + "Server__Host", null);
        }
    }

    [Fact]
    public void The_command_line_beats_everything()
    {
        var prefix = UniquePrefix("cli");
        Environment.SetEnvironmentVariable(prefix + "Server__Port", "9090");
        try
        {
            var config = LayeredConfig.Build(BaseJson, DevJson, prefix, ["--Server:Port=1234", "--Server:Host=0.0.0.0"]);

            Assert.Equal("1234", config["Server:Port"]);
            Assert.Equal("0.0.0.0", config["Server:Host"]);
        }
        finally
        {
            Environment.SetEnvironmentVariable(prefix + "Server__Port", null);
        }
    }

    [Fact]
    public void The_dump_is_sorted_and_hides_secrets()
    {
        var config = LayeredConfig.Build(BaseJson, DevJson, UniquePrefix("dump"), ["--Db:Token="]);

        Assert.Equal(
            string.Join('\n',
                "Db:ApiKey=***",
                "Db:Name=tasks",
                "Db:Password=***",
                "Db:Token=",
                "Logging:Level=Debug",
                "Server:Host=127.0.0.1",
                "Server:Port=5000"),
            LayeredConfig.DumpRedacted(config));
    }

    [Fact]
    public void Keys_that_merely_contain_key_are_not_redacted()
    {
        var config = LayeredConfig.Build("""{ "KeyboardLayout": "qwerty", "Jwt": { "SigningKey": "s" , "ClientSecret": "x" } }""", null, UniquePrefix("k"), []);

        Assert.Equal("Jwt:ClientSecret=***\nJwt:SigningKey=***\nKeyboardLayout=qwerty", LayeredConfig.DumpRedacted(config));
    }
}
