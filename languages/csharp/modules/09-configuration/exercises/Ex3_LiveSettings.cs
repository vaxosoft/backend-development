#pragma warning disable CS9113 // starter: the constructor parameter is for your implementation to use

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
    public LiveSettings(IOptionsMonitor<RateLimitOptions> monitor)
    {
        throw new NotImplementedException();
    }

    public RateLimitOptions Current => throw new NotImplementedException();

    public int Version => throw new NotImplementedException();

    public string? LastRejection => throw new NotImplementedException();

    public void Dispose()
    {
    }
}
