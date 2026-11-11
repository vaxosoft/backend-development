using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.Configuration.Memory;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Options;

namespace M09.Examples;

public sealed class FeatureOptions
{
    public bool BetaEnabled { get; set; }

    public int PageSize { get; set; } = 20;
}

/// <summary>Example 4: IOptions vs IOptionsSnapshot vs IOptionsMonitor when configuration changes.</summary>
public static class ReloadAndLifetimes
{
    public static Task<int> RunAsync(string[] args)
    {
        var memory = new MemoryConfigurationSource { InitialData = new Dictionary<string, string?> { ["Features:PageSize"] = "20" } };
        var root = new ConfigurationBuilder().Add(memory).Build();

        var services = new ServiceCollection();
        services.AddSingleton<IConfiguration>(root);
        services.AddOptions<FeatureOptions>().BindConfiguration("Features");
        using var provider = services.BuildServiceProvider();

        var once = provider.GetRequiredService<IOptions<FeatureOptions>>();
        var monitor = provider.GetRequiredService<IOptionsMonitor<FeatureOptions>>();
        using var subscription = monitor.OnChange((options, _) => Console.WriteLine($"  [monitor.OnChange] PageSize is now {options.PageSize}"));

        using (var scope = provider.CreateScope())
        {
            var snapshot = scope.ServiceProvider.GetRequiredService<IOptionsSnapshot<FeatureOptions>>();
            Console.WriteLine($"before change: IOptions={once.Value.PageSize} Snapshot={snapshot.Value.PageSize} Monitor={monitor.CurrentValue.PageSize}");
        }

        // Stand-in for "appsettings.json was edited" (file providers call Reload for you with reloadOnChange: true).
        root["Features:PageSize"] = "50";
        Console.WriteLine("-- configuration reloaded");
        root.Reload();

        using (var scope = provider.CreateScope())
        {
            var snapshot = scope.ServiceProvider.GetRequiredService<IOptionsSnapshot<FeatureOptions>>();
            Console.WriteLine($"after change:  IOptions={once.Value.PageSize} Snapshot={snapshot.Value.PageSize} Monitor={monitor.CurrentValue.PageSize}");
        }

        return Task.FromResult(0);
    }
}
