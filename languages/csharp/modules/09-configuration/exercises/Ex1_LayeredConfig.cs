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
        throw new NotImplementedException();
    }

    /// <summary>
    /// Returns one <c>Key=Value</c> line per configured value, sorted by key (ordinal, case-insensitive), joined by '\n'.
    /// Values of secret-looking keys are printed as <c>***</c>: the last key segment contains "password", "secret" or
    /// "token", or ends with "key" (case-insensitive). Empty values stay empty. Sections without a value are not listed.
    /// </summary>
    public static string DumpRedacted(IConfiguration config)
    {
        throw new NotImplementedException();
    }
}
