using Microsoft.Extensions.Options;

namespace M09.Exercises;

public sealed class RateLimitOptions
{
    public int PermitsPerMinute { get; set; } = 60;

    public int Burst { get; set; } = 10;
}

/// <summary>
/// Serves the rate-limit settings to the rest of the application and survives bad configuration edits.
/// <list type="bullet">
/// <item>A setting is valid when 1 &lt;= PermitsPerMinute &lt;= 10000 and 1 &lt;= Burst &lt;= PermitsPerMinute.</item>
/// <item>Construction reads <c>monitor.CurrentValue</c>; if it is invalid throw <see cref="InvalidOperationException"/> (fail fast).
/// <see cref="Current"/> then holds that value and <see cref="Version"/> is 1.</item>
/// <item>On every change notification: a valid value that differs from <see cref="Current"/> replaces it,
/// increments <see cref="Version"/> and clears <see cref="LastRejection"/>; an invalid value is ignored, <see cref="Current"/>
/// stays the last good one and <see cref="LastRejection"/> describes the problem (it names the offending property).
/// A change that leaves the values equal does nothing.</item>
/// <item>After <see cref="Dispose"/> notifications are ignored.</item>
/// </list>
/// </summary>
public sealed class LiveSettings : IDisposable
{
    private readonly IDisposable? _subscription;
    private readonly Lock _gate = new();
    private RateLimitOptions _current;
    private int _version = 1;
    private string? _lastRejection;

    public LiveSettings(IOptionsMonitor<RateLimitOptions> monitor)
    {
        var initial = Snapshot(monitor.CurrentValue);
        if (Validate(initial) is { } problem)
        {
            throw new InvalidOperationException(problem);
        }

        _current = initial;
        _subscription = monitor.OnChange(OnChanged);
    }

    public RateLimitOptions Current => Volatile.Read(ref _current);

    public int Version => Volatile.Read(ref _version);

    public string? LastRejection => Volatile.Read(ref _lastRejection);

    public void Dispose() => _subscription?.Dispose();

    private void OnChanged(RateLimitOptions value, string? name)
    {
        var candidate = Snapshot(value);
        lock (_gate)
        {
            if (Validate(candidate) is { } problem)
            {
                _lastRejection = problem;
                return;
            }

            _lastRejection = null;
            if (candidate.PermitsPerMinute != _current.PermitsPerMinute || candidate.Burst != _current.Burst)
            {
                _current = candidate;
                _version++;
            }
        }
    }

    // Copy so later mutations of the options instance cannot change what we already validated.
    private static RateLimitOptions Snapshot(RateLimitOptions o) => new() { PermitsPerMinute = o.PermitsPerMinute, Burst = o.Burst };

    private static string? Validate(RateLimitOptions o)
    {
        if (o.PermitsPerMinute is < 1 or > 10000)
        {
            return $"PermitsPerMinute must be 1-10000, got {o.PermitsPerMinute}";
        }

        return o.Burst < 1 || o.Burst > o.PermitsPerMinute
            ? $"Burst must be between 1 and PermitsPerMinute ({o.PermitsPerMinute}), got {o.Burst}"
            : null;
    }
}
