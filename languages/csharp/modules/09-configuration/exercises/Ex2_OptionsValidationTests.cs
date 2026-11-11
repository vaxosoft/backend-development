using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Options;

namespace M09.Exercises;

public class Ex2_OptionsValidationTests
{
    private static IHost BuildHost(Dictionary<string, string?> settings)
    {
        var builder = Host.CreateApplicationBuilder();
        builder.Logging.ClearProviders();
        builder.Configuration.AddInMemoryCollection(settings);
        builder.Services.AddAppOptions(builder.Configuration);
        return builder.Build();
    }

    [Fact]
    public async Task A_valid_section_is_bound_and_the_host_starts()
    {
        using var host = BuildHost(new()
        {
            ["App:Port"] = "9090",
            ["App:LogLevel"] = "warn",
            ["App:Environment"] = "Production",
            ["App:ConnectionString"] = "Data Source=tasks.db",
            ["App:RequestTimeout"] = "00:00:10",
        });

        await host.StartAsync();

        var options = host.Services.GetRequiredService<IOptions<AppOptions>>().Value;
        Assert.Equal(9090, options.Port);
        Assert.Equal("warn", options.LogLevel);
        Assert.Equal(TimeSpan.FromSeconds(10), options.RequestTimeout);
        await host.StopAsync();
    }

    [Fact]
    public async Task Defaults_are_valid_when_the_section_is_missing()
    {
        using var host = BuildHost([]);

        await host.StartAsync();

        Assert.Equal(8080, host.Services.GetRequiredService<IOptions<AppOptions>>().Value.Port);
        await host.StopAsync();
    }

    [Fact]
    public async Task An_invalid_section_stops_the_host_at_startup_and_lists_every_problem()
    {
        using var host = BuildHost(new()
        {
            ["App:Port"] = "70000",
            ["App:LogLevel"] = "verbose",
            ["App:Environment"] = "Production",
            ["App:RequestTimeout"] = "00:00:00",
        });

        var ex = await Assert.ThrowsAsync<OptionsValidationException>(() => host.StartAsync());

        var failures = ex.Failures.ToList();
        Assert.Contains(failures, f => f.Contains("Port"));
        Assert.Contains(failures, f => f.Contains("LogLevel"));
        Assert.Contains(failures, f => f.Contains("ConnectionString"));
        Assert.Contains(failures, f => f.Contains("RequestTimeout"));
        Assert.DoesNotContain(failures, f => f.Contains("Environment"));
    }

    [Theory]
    [InlineData(0, true)]
    [InlineData(1, false)]
    [InlineData(65535, false)]
    [InlineData(65536, true)]
    public void Port_boundaries(int port, bool fails)
    {
        var result = new AppOptionsValidator().Validate(null, new AppOptions { Port = port });

        Assert.Equal(fails, result.Failed);
    }

    [Theory]
    [InlineData("Development", null, false)]
    [InlineData("Staging", null, false)]
    [InlineData("Production", null, true)]
    [InlineData("Production", "  ", true)]
    [InlineData("Production", "Data Source=x", false)]
    [InlineData("Prod", "Data Source=x", true)]
    public void Environment_rules(string environment, string? connectionString, bool fails)
    {
        var result = new AppOptionsValidator().Validate(null, new AppOptions { Environment = environment, ConnectionString = connectionString });

        Assert.Equal(fails, result.Failed);
    }

    [Fact]
    public void Timeout_must_be_positive_and_at_most_five_minutes()
    {
        var validator = new AppOptionsValidator();

        Assert.True(validator.Validate(null, new AppOptions { RequestTimeout = TimeSpan.FromMinutes(5.1) }).Failed);
        Assert.True(validator.Validate(null, new AppOptions { RequestTimeout = TimeSpan.FromSeconds(-1) }).Failed);
        Assert.True(validator.Validate(null, new AppOptions { RequestTimeout = TimeSpan.FromMinutes(5) }).Succeeded);
    }
}
