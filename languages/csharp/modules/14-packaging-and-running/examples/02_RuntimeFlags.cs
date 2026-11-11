using Microsoft.Extensions.Configuration;

namespace M14.Examples;

public static class RuntimeFlags
{
    public sealed record ServerSettings(string Addr, string Environment, int ShutdownTimeoutSeconds, string LogLevel);

    /// <summary>Layers, lowest priority first: defaults, environment variables, command-line flags.</summary>
    public static ServerSettings Load(string[] args, IEnumerable<KeyValuePair<string, string?>>? environment = null)
    {
        var config = new ConfigurationBuilder()
            .AddInMemoryCollection(new Dictionary<string, string?>
            {
                ["ADDR"] = "127.0.0.1:8080",
                ["ENVIRONMENT"] = "Development",
                ["SHUTDOWN_TIMEOUT_SECONDS"] = "10",
                ["LOG_LEVEL"] = "Information",
            })
            .AddInMemoryCollection(environment ?? EnvironmentSubset())
            .AddCommandLine(args, new Dictionary<string, string>
            {
                ["--addr"] = "ADDR",
                ["--env"] = "ENVIRONMENT",
                ["--shutdown-timeout"] = "SHUTDOWN_TIMEOUT_SECONDS",
                ["--log-level"] = "LOG_LEVEL",
            })
            .Build();

        return new ServerSettings(
            config["ADDR"]!,
            config["ENVIRONMENT"]!,
            config.GetValue<int>("SHUTDOWN_TIMEOUT_SECONDS"),
            config["LOG_LEVEL"]!);
    }

    private static IEnumerable<KeyValuePair<string, string?>> EnvironmentSubset() =>
        new[] { "ADDR", "ENVIRONMENT", "SHUTDOWN_TIMEOUT_SECONDS", "LOG_LEVEL" }
            .Select(name => KeyValuePair.Create(name, Environment.GetEnvironmentVariable(name)))
            .Where(pair => pair.Value is not null);

    public static Task<int> RunAsync(string[] args)
    {
        var settings = Load(args);
        Console.WriteLine($"addr={settings.Addr}");
        Console.WriteLine($"environment={settings.Environment}");
        Console.WriteLine($"shutdown timeout={settings.ShutdownTimeoutSeconds}s");
        Console.WriteLine($"log level={settings.LogLevel}");
        return Task.FromResult(0);
    }
}
