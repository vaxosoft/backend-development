using System.Collections.Concurrent;
using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Hosting.Server;
using Microsoft.AspNetCore.Hosting.Server.Features;
using Microsoft.AspNetCore.Http;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Logging;

namespace P4.ApiAggregator.Tests;

/// <summary>What the fake upstream does for one call.</summary>
public sealed record Behavior(int Status = 200, int DelayMs = 0, bool Abort = false);

/// <summary>
/// A real Kestrel server on a loopback port. It counts calls and concurrent calls and can be scripted per
/// (id, call number) to fail, stall or drop the connection.
/// </summary>
public sealed class FakeUpstream : IAsyncDisposable
{
    private readonly WebApplication _app;
    private readonly ConcurrentDictionary<int, int> _calls = new();
    private int _inFlight;
    private int _maxInFlight;
    private int _cancelled;

    private FakeUpstream(WebApplication app, string baseUrl)
    {
        _app = app;
        BaseUrl = baseUrl;
    }

    public string BaseUrl { get; }

    /// <summary>Decides the behaviour from (id, 1-based call number for that id).</summary>
    public Func<int, int, Behavior> Script { get; set; } = (_, _) => new Behavior();

    public int MaxInFlight => Volatile.Read(ref _maxInFlight);

    /// <summary>Calls whose connection was dropped by the caller while the upstream was still working.</summary>
    public int Cancelled => Volatile.Read(ref _cancelled);

    public int InFlight => Volatile.Read(ref _inFlight);

    public int Calls(int id) => _calls.GetValueOrDefault(id);

    public static async Task<FakeUpstream> StartAsync()
    {
        var builder = WebApplication.CreateBuilder();
        builder.Logging.ClearProviders();
        builder.WebHost.UseUrls("http://127.0.0.1:0");
        var app = builder.Build();
        FakeUpstream? fake = null;

        app.MapGet("/items/{id:int}", async (int id, HttpContext context) =>
        {
            var call = fake!._calls.AddOrUpdate(id, 1, (_, n) => n + 1);
            var behavior = fake.Script(id, call);
            fake.Enter();
            try
            {
                if (behavior.DelayMs > 0)
                {
                    await Task.Delay(behavior.DelayMs, context.RequestAborted);
                }

                if (behavior.Abort)
                {
                    context.Abort();
                    return Results.Empty;
                }

                return behavior.Status == 200
                    ? Results.Json(new { id, name = $"item-{id}" })
                    : Results.StatusCode(behavior.Status);
            }
            catch (OperationCanceledException)
            {
                Interlocked.Increment(ref fake._cancelled);
                return Results.Empty;
            }
            finally
            {
                Interlocked.Decrement(ref fake._inFlight);
            }
        });

        await app.StartAsync();
        var address = app.Services.GetRequiredService<IServer>().Features.Get<IServerAddressesFeature>()!.Addresses.First();
        fake = new FakeUpstream(app, address);
        return fake;
    }

    private void Enter()
    {
        var now = Interlocked.Increment(ref _inFlight);
        int seen;
        while (now > (seen = Volatile.Read(ref _maxInFlight)) && Interlocked.CompareExchange(ref _maxInFlight, now, seen) != seen)
        {
        }
    }

    public async ValueTask DisposeAsync()
    {
        await _app.StopAsync();
        await _app.DisposeAsync();
    }
}
