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
    public static ServerOptions Parse(string[] args, IReadOnlyDictionary<string, string> environment) =>
        throw new NotImplementedException();
}
