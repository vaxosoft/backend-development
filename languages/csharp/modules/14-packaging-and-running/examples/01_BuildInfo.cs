using System.Reflection;

namespace M14.Examples;

public static class BuildInfoDemo
{
    /// <summary>The version stamped into the assembly at build time: <c>Version</c> plus optional <c>+commit</c>.</summary>
    public static string InformationalVersion() =>
        Assembly.GetExecutingAssembly().GetCustomAttribute<AssemblyInformationalVersionAttribute>()?.InformationalVersion ?? "unknown";

    public static Task<int> RunAsync(string[] args)
    {
        var assembly = Assembly.GetExecutingAssembly().GetName();
        var version = InformationalVersion();
        var plus = version.IndexOf('+');

        Console.WriteLine($"assembly: {assembly.Name}");
        Console.WriteLine($"informational version: {version}");
        Console.WriteLine($"version part: {(plus < 0 ? version : version[..plus])}");
        Console.WriteLine($"commit part: {(plus < 0 ? "(none)" : version[(plus + 1)..])}");
        return Task.FromResult(0);
    }
}
