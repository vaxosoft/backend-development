using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;

namespace M09.Examples;

public sealed record AppConfig(int Port, string LogLevel, string Environment, string JwtSecret)
{
    private static readonly string[] LogLevels = ["debug", "info", "warn", "error"];
    private static readonly string[] Environments = ["development", "staging", "production"];

    /// <summary>Validates everything at once and returns all problems; startup code decides what to do with them.</summary>
    public static bool TryLoad(IConfiguration config, out AppConfig? result, out List<string> errors)
    {
        errors = [];
        var portText = config["PORT"] ?? "8080";
        if (!int.TryParse(portText, out var port) || port is < 1 or > 65535)
        {
            errors.Add($"PORT must be an integer between 1 and 65535, got '{portText}'");
        }

        var level = (config["LOG_LEVEL"] ?? "info").ToLowerInvariant();
        if (!LogLevels.Contains(level))
        {
            errors.Add($"LOG_LEVEL must be one of {string.Join('|', LogLevels)}, got '{level}'");
        }

        var env = (config["ENV"] ?? "development").ToLowerInvariant();
        if (!Environments.Contains(env))
        {
            errors.Add($"ENV must be one of {string.Join('|', Environments)}, got '{env}'");
        }

        var secret = config["JWT_SECRET"] ?? "";
        if (env == "production" && secret.Length < 16)
        {
            errors.Add("JWT_SECRET must be at least 16 characters in production");
        }

        result = errors.Count == 0 ? new AppConfig(port, level, env, secret) : null;
        return errors.Count == 0;
    }

    /// <summary>Safe to log: secrets are replaced by a marker.</summary>
    public override string ToString() => $"Port={Port} LogLevel={LogLevel} Environment={Environment} JwtSecret={(JwtSecret.Length == 0 ? "<unset>" : "***")}";
}

public interface ITaskStore
{
    int Count { get; }
}

public sealed class InMemoryTaskStore : ITaskStore
{
    public int Count => 3;
}

public sealed class TaskService(ITaskStore store, AppConfig config)
{
    public string Describe() => $"TaskService with {store.Count} tasks, listening on :{config.Port}";
}

/// <summary>Example 5: a validated, typed AppConfig loaded from the environment, wired by hand and with DI.</summary>
public static class TaskTrackerConfig
{
    public static Task<int> RunAsync(string[] args)
    {
        Console.WriteLine("-- invalid environment");
        var invalid = FromEnvironment(("PORT", "http"), ("LOG_LEVEL", "verbose"), ("ENV", "production"));
        if (!AppConfig.TryLoad(invalid, out _, out var errors))
        {
            foreach (var error in errors)
            {
                Console.WriteLine($"config error: {error}");
            }
        }

        Console.WriteLine("-- valid environment");
        var valid = FromEnvironment(("PORT", "9090"), ("LOG_LEVEL", "WARN"), ("ENV", "production"), ("JWT_SECRET", "correct-horse-battery-staple"));
        if (!AppConfig.TryLoad(valid, out var config, out _))
        {
            return Task.FromResult(1);
        }

        Console.WriteLine($"loaded: {config}");

        Console.WriteLine("-- wiring by hand (composition root)");
        Console.WriteLine(new TaskService(new InMemoryTaskStore(), config!).Describe());

        Console.WriteLine("-- wiring with the DI container");
        var services = new ServiceCollection();
        services.AddSingleton(config!);
        services.AddSingleton<ITaskStore, InMemoryTaskStore>();
        services.AddSingleton<TaskService>();
        using var provider = services.BuildServiceProvider(new ServiceProviderOptions { ValidateOnBuild = true, ValidateScopes = true });
        Console.WriteLine(provider.GetRequiredService<TaskService>().Describe());
        return Task.FromResult(0);
    }

    private static IConfiguration FromEnvironment(params (string Key, string Value)[] variables)
    {
        // Unique prefix per call so the demo does not depend on the real environment.
        var prefix = $"TTDEMO{Guid.NewGuid():N}_";
        foreach (var (key, value) in variables)
        {
            Environment.SetEnvironmentVariable(prefix + key, value);
        }

        return new ConfigurationBuilder().AddEnvironmentVariables(prefix).Build();
    }
}
