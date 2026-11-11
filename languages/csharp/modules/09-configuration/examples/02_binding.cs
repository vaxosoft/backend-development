using Microsoft.Extensions.Configuration;

namespace M09.Examples;

public enum LogLevelName { Debug, Information, Warning, Error }

public sealed class ServerOptions
{
    public string Host { get; set; } = "127.0.0.1";

    public int Port { get; set; } = 8080;

    public TimeSpan RequestTimeout { get; set; } = TimeSpan.FromSeconds(30);

    public LogLevelName LogLevel { get; set; } = LogLevelName.Information;

    public string[] AllowedOrigins { get; set; } = [];

    public Dictionary<string, int> RateLimits { get; set; } = [];
}

/// <summary>Example 2: binding configuration sections to typed objects, and what goes quietly wrong.</summary>
public static class Binding
{
    public static Task<int> RunAsync(string[] args)
    {
        var config = new ConfigurationBuilder()
            .AddInMemoryCollection(new Dictionary<string, string?>
            {
                ["Server:Host"] = "0.0.0.0",
                ["Server:Port"] = "9090",
                ["Server:RequestTimeout"] = "00:00:05",
                ["Server:LogLevel"] = "warning",
                ["Server:AllowedOrigins:0"] = "https://app.example.com",
                ["Server:AllowedOrigins:1"] = "https://admin.example.com",
                ["Server:RateLimits:login"] = "5",
                ["Server:RateLimits:search"] = "30",
            })
            .Build();

        var server = config.GetSection("Server").Get<ServerOptions>()!;
        Console.WriteLine($"host={server.Host} port={server.Port} timeout={server.RequestTimeout} level={server.LogLevel}");
        Console.WriteLine($"origins=[{string.Join(", ", server.AllowedOrigins)}] limits=[{string.Join(", ", server.RateLimits.Select(p => $"{p.Key}:{p.Value}"))}]");
        Console.WriteLine($"GetValue<int>: {config.GetValue<int>("Server:Port")}, with default: {config.GetValue("Server:Missing", 42)}");

        Console.WriteLine();
        Console.WriteLine("-- a typo is silently ignored");
        var typoWithServer = new ConfigurationBuilder()
            .AddInMemoryCollection(new Dictionary<string, string?> { ["Server:Host"] = "h", ["Server:Prot"] = "9090" }).Build();
        var bound = typoWithServer.GetSection("Server").Get<ServerOptions>()!;
        Console.WriteLine($"Port after typo 'Prot': {bound.Port} (the default; no error)");

        try
        {
            typoWithServer.GetSection("Server").Get<ServerOptions>(o => o.ErrorOnUnknownConfiguration = true);
        }
        catch (InvalidOperationException ex)
        {
            Console.WriteLine($"ErrorOnUnknownConfiguration: {ex.Message.ReplaceLineEndings(" ").Trim()}");
        }

        Console.WriteLine();
        Console.WriteLine("-- a value of the wrong type is loud");
        var bad = new ConfigurationBuilder().AddInMemoryCollection(new Dictionary<string, string?> { ["Server:Port"] = "eighty" }).Build();
        try
        {
            bad.GetSection("Server").Get<ServerOptions>();
        }
        catch (InvalidOperationException ex)
        {
            Console.WriteLine($"InvalidOperationException: {ex.Message.ReplaceLineEndings("\n").Split('\n')[0]}");
        }

        return Task.FromResult(0);
    }
}
