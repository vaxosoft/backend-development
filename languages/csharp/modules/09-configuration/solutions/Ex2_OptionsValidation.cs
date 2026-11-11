using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Options;

namespace M09.Exercises;

public sealed class AppOptions
{
    public int Port { get; set; } = 8080;

    public string LogLevel { get; set; } = "info";

    public string Environment { get; set; } = "Development";

    public string? ConnectionString { get; set; }

    public TimeSpan RequestTimeout { get; set; } = TimeSpan.FromSeconds(30);
}

/// <summary>
/// Collects EVERY problem (do not stop at the first) as a failure message that names the offending property:
/// <list type="bullet">
/// <item>Port: 1-65535</item>
/// <item>LogLevel: one of debug, info, warn, error (lowercase)</item>
/// <item>Environment: Development, Staging or Production</item>
/// <item>ConnectionString: required (not null/empty/whitespace) when Environment is Production</item>
/// <item>RequestTimeout: greater than zero and at most 5 minutes</item>
/// </list>
/// Return <see cref="ValidateOptionsResult.Success"/> when everything is fine.
/// </summary>
public sealed class AppOptionsValidator : IValidateOptions<AppOptions>
{
    private static readonly string[] LogLevels = ["debug", "info", "warn", "error"];
    private static readonly string[] Environments = ["Development", "Staging", "Production"];

    public ValidateOptionsResult Validate(string? name, AppOptions options)
    {
        List<string> failures = [];
        if (options.Port is < 1 or > 65535)
        {
            failures.Add($"Port must be between 1 and 65535, got {options.Port}");
        }

        if (!LogLevels.Contains(options.LogLevel))
        {
            failures.Add($"LogLevel must be one of {string.Join(", ", LogLevels)}, got '{options.LogLevel}'");
        }

        if (!Environments.Contains(options.Environment))
        {
            failures.Add($"Environment must be one of {string.Join(", ", Environments)}, got '{options.Environment}'");
        }

        if (options.Environment == "Production" && string.IsNullOrWhiteSpace(options.ConnectionString))
        {
            failures.Add("ConnectionString is required in Production");
        }

        if (options.RequestTimeout <= TimeSpan.Zero || options.RequestTimeout > TimeSpan.FromMinutes(5))
        {
            failures.Add($"RequestTimeout must be greater than zero and at most 5 minutes, got {options.RequestTimeout}");
        }

        return failures.Count == 0 ? ValidateOptionsResult.Success : ValidateOptionsResult.Fail(failures);
    }
}

public static class AppOptionsSetup
{
    /// <summary>
    /// Registers <see cref="AppOptions"/> bound to the "App" section of <paramref name="configuration"/>,
    /// validated by <see cref="AppOptionsValidator"/>, and validated eagerly when the host starts.
    /// </summary>
    public static IServiceCollection AddAppOptions(this IServiceCollection services, IConfiguration configuration)
    {
        services.AddSingleton<IValidateOptions<AppOptions>, AppOptionsValidator>();
        services.AddOptions<AppOptions>()
            .Bind(configuration.GetSection("App"))
            .ValidateOnStart();
        return services;
    }
}
