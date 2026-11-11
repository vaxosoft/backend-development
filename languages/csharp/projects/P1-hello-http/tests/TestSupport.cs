using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Mvc.Testing;
using Microsoft.AspNetCore.TestHost;
using Microsoft.Extensions.DependencyInjection;
using P1.HelloHttp;

namespace P1.HelloHttp.Tests;

public sealed class CapturingSink : IRequestLogSink
{
    private readonly List<RequestLogEntry> _entries = [];

    public IReadOnlyList<RequestLogEntry> Entries
    {
        get
        {
            lock (_entries)
            {
                return [.. _entries];
            }
        }
    }

    public void Write(RequestLogEntry entry)
    {
        lock (_entries)
        {
            _entries.Add(entry);
        }
    }
}

/// <summary>Records the delays asked for and makes them instant.</summary>
public sealed class RecordingTimeProvider : TimeProvider
{
    public List<TimeSpan> Delays { get; } = [];

    public override ITimer CreateTimer(TimerCallback callback, object? state, TimeSpan dueTime, TimeSpan period)
    {
        lock (Delays)
        {
            Delays.Add(dueTime);
        }

        return base.CreateTimer(callback, state, TimeSpan.Zero, period);
    }
}

public sealed class HelloFactory : WebApplicationFactory<Program>
{
    public CapturingSink Sink { get; } = new();

    public RecordingTimeProvider Time { get; } = new();

    protected override void ConfigureWebHost(IWebHostBuilder builder) =>
        builder.ConfigureTestServices(services =>
        {
            services.AddSingleton<IRequestLogSink>(Sink);
            services.AddSingleton<TimeProvider>(Time);
        });
}
