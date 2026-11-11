using System.Globalization;

namespace P7.TestedService;

/// <summary>Validated process configuration.</summary>
public sealed record AppConfig(int Port, LogLevel LogLevel, string Env)
{
    public static readonly string[] Environments = ["dev", "staging", "prod"];

    /// <summary>
    /// Reads <c>PORT</c> (1-65535, default 8080), <c>LOG_LEVEL</c> (debug|info|warn|error, default info) and
    /// <c>ENV</c> (dev|staging|prod, default dev). Unset or blank means default; anything else invalid is reported,
    /// and all problems are collected so the operator can fix them in one go.
    /// </summary>
    public static ConfigResult Parse(Func<string, string?> get)
    {
        var errors = new List<string>();

        var port = 8080;
        if (Read(get, "PORT") is { } rawPort
            && !(int.TryParse(rawPort, NumberStyles.None, CultureInfo.InvariantCulture, out port) && port is >= 1 and <= 65535))
        {
            errors.Add($"PORT must be an integer between 1 and 65535 (got '{rawPort}')");
        }

        var logLevel = LogLevel.Information;
        if (Read(get, "LOG_LEVEL") is { } rawLevel)
        {
            switch (rawLevel.ToLowerInvariant())
            {
                case "debug":
                    logLevel = LogLevel.Debug;
                    break;
                case "info":
                    break;
                case "warn":
                    logLevel = LogLevel.Warning;
                    break;
                case "error":
                    logLevel = LogLevel.Error;
                    break;
                default:
                    errors.Add($"LOG_LEVEL must be one of debug, info, warn, error (got '{rawLevel}')");
                    break;
            }
        }

        var env = "dev";
        if (Read(get, "ENV") is { } rawEnv)
        {
            env = rawEnv.ToLowerInvariant();
            if (!Environments.Contains(env))
            {
                errors.Add($"ENV must be one of {string.Join(", ", Environments)} (got '{rawEnv}')");
            }
        }

        return errors.Count == 0
            ? new ConfigResult(new AppConfig(port, logLevel, env), [])
            : new ConfigResult(null, errors);
    }

    private static string? Read(Func<string, string?> get, string key) =>
        get(key) is { } value && !string.IsNullOrWhiteSpace(value) ? value.Trim() : null;
}

/// <summary>Either a valid config or the list of everything that is wrong with the environment.</summary>
public sealed record ConfigResult(AppConfig? Config, IReadOnlyList<string> Errors)
{
    public bool IsValid => Errors.Count == 0;
}
