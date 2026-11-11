using System.Globalization;

namespace M14.Exercises;

public sealed class FlagException(string message) : Exception(message);

public sealed record ServerOptions(string Host, int Port, string Environment, TimeSpan ShutdownTimeout);

/// <summary>
/// Resolves runtime settings with the precedence flag > environment variable > default.
/// <list type="table">
/// <item><term>--addr host:port</term><description>env ADDR, default 127.0.0.1:8080 (port 1-65535; IPv6 hosts like [::1] allowed)</description></item>
/// <item><term>--env name</term><description>env ENVIRONMENT, default Development; one of Development, Staging, Production (case-insensitive, returned in canonical case)</description></item>
/// <item><term>--shutdown-timeout seconds</term><description>env SHUTDOWN_TIMEOUT_SECONDS, default 10; integer 0-300</description></item>
/// </list>
/// Flags may be written <c>--name value</c> or <c>--name=value</c>. Unknown flags, missing values and invalid values throw
/// <see cref="FlagException"/> whose message names the offending flag or variable.
/// </summary>
public static class Ex2_ServerOptionsParser
{
    private static readonly string[] Environments = ["Development", "Staging", "Production"];

    private static readonly Dictionary<string, string> EnvNames = new()
    {
        ["--addr"] = "ADDR",
        ["--env"] = "ENVIRONMENT",
        ["--shutdown-timeout"] = "SHUTDOWN_TIMEOUT_SECONDS",
    };

    public static ServerOptions Parse(string[] args, IReadOnlyDictionary<string, string> environment)
    {
        var flags = ReadFlags(args);

        string Pick(string flag, string fallback) =>
            flags.TryGetValue(flag, out var value) ? value
            : environment.TryGetValue(EnvNames[flag], out var fromEnv) ? fromEnv
            : fallback;

        string Source(string flag) => flags.ContainsKey(flag) ? flag : EnvNames[flag];

        var (host, port) = ParseAddr(Pick("--addr", "127.0.0.1:8080"), Source("--addr"));
        var name = Environments.FirstOrDefault(e => e.Equals(Pick("--env", "Development"), StringComparison.OrdinalIgnoreCase))
            ?? throw new FlagException($"{Source("--env")} must be one of {string.Join(", ", Environments)}");
        var seconds = int.TryParse(Pick("--shutdown-timeout", "10"), NumberStyles.None, CultureInfo.InvariantCulture, out var s) && s <= 300
            ? s
            : throw new FlagException($"{Source("--shutdown-timeout")} must be an integer from 0 to 300");

        return new ServerOptions(host, port, name, TimeSpan.FromSeconds(seconds));
    }

    private static Dictionary<string, string> ReadFlags(string[] args)
    {
        var flags = new Dictionary<string, string>();
        for (var i = 0; i < args.Length; i++)
        {
            var arg = args[i];
            var equals = arg.IndexOf('=');
            var name = equals < 0 ? arg : arg[..equals];
            if (!EnvNames.ContainsKey(name))
            {
                throw new FlagException($"unknown flag {name}");
            }

            if (equals >= 0)
            {
                flags[name] = arg[(equals + 1)..];
            }
            else if (i + 1 < args.Length)
            {
                flags[name] = args[++i];
            }
            else
            {
                throw new FlagException($"flag {name} needs a value");
            }
        }

        return flags;
    }

    private static (string Host, int Port) ParseAddr(string addr, string source)
    {
        var colon = addr.LastIndexOf(':');
        if (colon <= 0
            || !int.TryParse(addr[(colon + 1)..], NumberStyles.None, CultureInfo.InvariantCulture, out var port)
            || port is < 1 or > 65535)
        {
            throw new FlagException($"{source} must look like host:port with a port from 1 to 65535");
        }

        return (addr[..colon], port);
    }
}
