using System.Text;
using Microsoft.Extensions.Configuration;

namespace M09.Exercises;

public static class LayeredConfig
{
    /// <summary>
    /// Builds a configuration from four layers; a later layer overrides an earlier one:
    /// <list type="number">
    /// <item><paramref name="baseJson"/> (JSON text, skipped when null)</item>
    /// <item><paramref name="environmentJson"/> (JSON text, skipped when null)</item>
    /// <item>environment variables that start with <paramref name="envPrefix"/> (<c>PREFIX_Server__Port</c> means <c>Server:Port</c>)</item>
    /// <item>command-line arguments (<c>--Server:Port=1</c>)</item>
    /// </list>
    /// </summary>
    public static IConfigurationRoot Build(string? baseJson, string? environmentJson, string envPrefix, string[] args)
    {
        var builder = new ConfigurationBuilder();
        if (baseJson is not null)
        {
            builder.AddJsonStream(new MemoryStream(Encoding.UTF8.GetBytes(baseJson)));
        }

        if (environmentJson is not null)
        {
            builder.AddJsonStream(new MemoryStream(Encoding.UTF8.GetBytes(environmentJson)));
        }

        return builder.AddEnvironmentVariables(envPrefix).AddCommandLine(args).Build();
    }

    /// <summary>
    /// Returns one <c>Key=Value</c> line per configured value, sorted by key (ordinal, case-insensitive), joined by '\n'.
    /// Values of secret-looking keys are printed as <c>***</c>: the last key segment contains "password", "secret" or
    /// "token", or ends with "key" (case-insensitive). Empty values stay empty. Sections without a value are not listed.
    /// </summary>
    public static string DumpRedacted(IConfiguration config)
    {
        var lines = config.AsEnumerable()
            .Where(pair => pair.Value is not null)
            .OrderBy(pair => pair.Key, StringComparer.OrdinalIgnoreCase)
            .Select(pair => $"{pair.Key}={(pair.Value!.Length > 0 && IsSecret(pair.Key) ? "***" : pair.Value)}");
        return string.Join('\n', lines);
    }

    private static bool IsSecret(string key)
    {
        var last = key[(key.LastIndexOf(':') + 1)..];
        return last.Contains("password", StringComparison.OrdinalIgnoreCase)
            || last.Contains("secret", StringComparison.OrdinalIgnoreCase)
            || last.Contains("token", StringComparison.OrdinalIgnoreCase)
            || last.EndsWith("key", StringComparison.OrdinalIgnoreCase);
    }
}
