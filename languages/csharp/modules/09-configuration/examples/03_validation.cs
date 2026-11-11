using System.ComponentModel.DataAnnotations;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Options;

namespace M09.Examples;

public sealed class AppOptions
{
    [Range(1, 65535)]
    public int Port { get; set; } = 8080;

    [Required, RegularExpression("^(debug|info|warn|error)$")]
    public string LogLevel { get; set; } = "info";

    public string? ConnectionString { get; set; }

    public string Environment { get; set; } = "Development";
}

/// <summary>Rules that need more than attributes (cross-property or environment dependent).</summary>
public sealed class AppOptionsRules : IValidateOptions<AppOptions>
{
    public ValidateOptionsResult Validate(string? name, AppOptions options) =>
        options is { Environment: "Production", ConnectionString: null or "" }
            ? ValidateOptionsResult.Fail("ConnectionString is required when Environment is Production")
            : ValidateOptionsResult.Success;
}

/// <summary>Example 3: fail at startup, with every problem listed, instead of at the first request.</summary>
public static class Validation
{
    public static async Task<int> RunAsync(string[] args)
    {
        await StartAsync("broken config", new() { ["App:Port"] = "70000", ["App:LogLevel"] = "verbose", ["App:Environment"] = "Production" });
        await StartAsync("good config", new() { ["App:Port"] = "9090", ["App:LogLevel"] = "warn", ["App:Environment"] = "Production", ["App:ConnectionString"] = "Data Source=tasks.db" });
        return 0;
    }

    private static async Task StartAsync(string label, Dictionary<string, string?> settings)
    {
        var builder = Host.CreateApplicationBuilder();
        builder.Logging.ClearProviders();
        builder.Configuration.AddInMemoryCollection(settings);
        builder.Services.AddOptions<AppOptions>()
            .BindConfiguration("App")
            .ValidateDataAnnotations()
            .ValidateOnStart();
        builder.Services.AddSingleton<IValidateOptions<AppOptions>, AppOptionsRules>();

        using var host = builder.Build();
        Console.WriteLine($"-- {label}");
        try
        {
            await host.StartAsync();
            var options = host.Services.GetRequiredService<IOptions<AppOptions>>().Value;
            Console.WriteLine($"started: port={options.Port} level={options.LogLevel} env={options.Environment}");
            await host.StopAsync();
        }
        catch (OptionsValidationException ex)
        {
            Console.WriteLine($"startup failed ({ex.Failures.Count()} problems):");
            foreach (var failure in ex.Failures)
            {
                Console.WriteLine($"  - {failure}");
            }
        }
    }
}
