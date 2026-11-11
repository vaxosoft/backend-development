using Microsoft.Extensions.Configuration;

namespace M09.Examples;

/// <summary>Example 1: providers are layers; the last one that knows a key wins.</summary>
public static class Layers
{
    public static Task<int> RunAsync(string[] args)
    {
        // Real deployments set these outside the process; we set them here so the output is reproducible.
        Environment.SetEnvironmentVariable("TT_Server__Port", "9090");
        Environment.SetEnvironmentVariable("TT_Database__ConnectionString", "Data Source=/var/lib/tt/tasks.db");
        string[] commandLine = ["--Server:Host=0.0.0.0", .. args];

        var environmentName = Environment.GetEnvironmentVariable("TT_ENVIRONMENT") ?? "Development";

        var config = new ConfigurationBuilder()
            .SetBasePath(Path.Combine(AppContext.BaseDirectory, "config"))
            .AddInMemoryCollection(new Dictionary<string, string?> { ["Server:Port"] = "1", ["Feature:Beta"] = "false" })
            .AddJsonFile("appsettings.json", optional: false)
            .AddJsonFile($"appsettings.{environmentName}.json", optional: true)
            .AddEnvironmentVariables(prefix: "TT_")
            .AddCommandLine(commandLine)
            .Build();

        // Layer order: defaults < appsettings.json < appsettings.<Environment>.json < environment < command line
        Console.WriteLine($"Server:Host         = {config["Server:Host"]}   (command line beat appsettings.json)");
        Console.WriteLine($"Server:Port         = {config["Server:Port"]}   (environment beat Development file, JSON and defaults)");
        Console.WriteLine($"Logging:Level       = {config["Logging:Level"]}   (Development file beat appsettings.json)");
        Console.WriteLine($"Server:RequestTimeout = {config["Server:RequestTimeout"]}   (only appsettings.json knows it)");
        Console.WriteLine($"Feature:Beta        = {config["Feature:Beta"]}   (only the in-memory defaults know it)");
        Console.WriteLine($"Not:There           = {config["Not:There"] ?? "<null>"}   (missing keys are null, not errors)");

        Console.WriteLine();
        Console.WriteLine("Who supplied what:");
        Console.WriteLine(config.GetDebugView());
        return Task.FromResult(0);
    }
}
