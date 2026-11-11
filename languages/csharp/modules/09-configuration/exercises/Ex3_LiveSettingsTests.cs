using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.Configuration.Memory;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Options;

namespace M09.Exercises;

public class Ex3_LiveSettingsTests
{
    private sealed class Fixture : IDisposable
    {
        private readonly ServiceProvider _provider;

        public Fixture(int permits, int burst)
        {
            Root = new ConfigurationBuilder()
                .Add(new MemoryConfigurationSource
                {
                    InitialData = new Dictionary<string, string?>
                    {
                        ["RateLimit:PermitsPerMinute"] = permits.ToString(),
                        ["RateLimit:Burst"] = burst.ToString(),
                    },
                })
                .Build();
            var services = new ServiceCollection();
            services.AddOptions<RateLimitOptions>().Bind(Root.GetSection("RateLimit"));
            _provider = services.BuildServiceProvider();
            Monitor = _provider.GetRequiredService<IOptionsMonitor<RateLimitOptions>>();
        }

        public IConfigurationRoot Root { get; }

        public IOptionsMonitor<RateLimitOptions> Monitor { get; }

        public void Edit(int permits, int burst)
        {
            Root["RateLimit:PermitsPerMinute"] = permits.ToString();
            Root["RateLimit:Burst"] = burst.ToString();
            Root.Reload();
        }

        public void Dispose() => _provider.Dispose();
    }

    [Fact]
    public void Starts_with_the_initial_values()
    {
        using var f = new Fixture(120, 20);
        using var settings = new LiveSettings(f.Monitor);

        Assert.Equal(120, settings.Current.PermitsPerMinute);
        Assert.Equal(20, settings.Current.Burst);
        Assert.Equal(1, settings.Version);
        Assert.Null(settings.LastRejection);
    }

    [Fact]
    public void Invalid_initial_configuration_fails_fast()
    {
        using var f = new Fixture(10, 50);

        Assert.Throws<InvalidOperationException>(() => new LiveSettings(f.Monitor));
    }

    [Fact]
    public void A_valid_edit_is_picked_up_without_a_restart()
    {
        using var f = new Fixture(60, 10);
        using var settings = new LiveSettings(f.Monitor);

        f.Edit(600, 100);

        Assert.Equal(600, settings.Current.PermitsPerMinute);
        Assert.Equal(100, settings.Current.Burst);
        Assert.Equal(2, settings.Version);
    }

    [Fact]
    public void An_invalid_edit_is_rejected_and_the_last_good_values_stay()
    {
        using var f = new Fixture(60, 10);
        using var settings = new LiveSettings(f.Monitor);

        f.Edit(5, 99);

        Assert.Equal(60, settings.Current.PermitsPerMinute);
        Assert.Equal(1, settings.Version);
        Assert.Contains("Burst", settings.LastRejection);

        f.Edit(0, 1);
        Assert.Contains("PermitsPerMinute", settings.LastRejection);
        Assert.Equal(60, settings.Current.PermitsPerMinute);
    }

    [Fact]
    public void A_later_valid_edit_recovers_and_clears_the_rejection()
    {
        using var f = new Fixture(60, 10);
        using var settings = new LiveSettings(f.Monitor);
        f.Edit(5, 99);

        f.Edit(300, 30);

        Assert.Equal(300, settings.Current.PermitsPerMinute);
        Assert.Equal(2, settings.Version);
        Assert.Null(settings.LastRejection);
    }

    [Fact]
    public void A_reload_that_changes_nothing_is_not_a_new_version()
    {
        using var f = new Fixture(60, 10);
        using var settings = new LiveSettings(f.Monitor);

        f.Root.Reload();
        f.Edit(60, 10);

        Assert.Equal(1, settings.Version);
    }

    [Fact]
    public void Disposed_settings_stop_listening()
    {
        using var f = new Fixture(60, 10);
        var settings = new LiveSettings(f.Monitor);
        settings.Dispose();

        f.Edit(600, 100);

        Assert.Equal(60, settings.Current.PermitsPerMinute);
        Assert.Equal(1, settings.Version);
    }
}
