namespace P7.TestedService;

/// <summary>Validated process configuration.</summary>
public sealed record AppConfig(int Port, LogLevel LogLevel, string Env)
{
    public static readonly string[] Environments = ["dev", "staging", "prod"];

    /// <summary>
    /// Reads <c>PORT</c> (1-65535, default 8080), <c>LOG_LEVEL</c> (debug|info|warn|error, default info) and
    /// <c>ENV</c> (dev|staging|prod, default dev). Unset or blank means default (see the project README).
    /// </summary>
    public static ConfigResult Parse(Func<string, string?> get)
    {
        // TODO: read PORT, LOG_LEVEL and ENV with `get`, apply the defaults, and return every problem found as an
        // error message that names the variable and the bad value (Errors non-empty => Config is null).
        return new ConfigResult(new AppConfig(8080, LogLevel.Information, "dev"), []);
    }
}

/// <summary>Either a valid config or the list of everything that is wrong with the environment.</summary>
public sealed record ConfigResult(AppConfig? Config, IReadOnlyList<string> Errors)
{
    public bool IsValid => Errors.Count == 0;
}
