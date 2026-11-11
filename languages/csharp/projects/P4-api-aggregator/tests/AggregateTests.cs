using System.Diagnostics;
using System.Net;
using System.Text.Json;
using Microsoft.AspNetCore.Mvc.Testing;
using Microsoft.Extensions.DependencyInjection;

namespace P4.ApiAggregator.Tests;

public sealed class AggregateTests : IAsyncLifetime
{
    private FakeUpstream _upstream = null!;

    public async Task InitializeAsync() => _upstream = await FakeUpstream.StartAsync();

    public async Task DisposeAsync() => await _upstream.DisposeAsync();

    private WebApplicationFactory<Program> StartApp(Action<AggregatorOptions>? configure = null) =>
        new WebApplicationFactory<Program>().WithWebHostBuilder(builder => builder.ConfigureServices(services =>
            services.PostConfigure<AggregatorOptions>(o =>
            {
                o.UpstreamBaseUrl = _upstream.BaseUrl;
                o.BackoffBaseMs = 1;
                configure?.Invoke(o);
            })));

    private static async Task<(HttpResponseMessage Response, JsonElement Body)> GetAsync(HttpClient client, string url)
    {
        var response = await client.GetAsync(url);
        var text = await response.Content.ReadAsStringAsync();
        var body = response.Content.Headers.ContentType?.MediaType?.EndsWith("json") == true
            ? JsonDocument.Parse(text).RootElement.Clone()
            : default;
        return (response, body);
    }

    private static async Task Eventually(Func<bool> condition)
    {
        var deadline = Stopwatch.StartNew();
        while (!condition())
        {
            Assert.True(deadline.Elapsed < TimeSpan.FromSeconds(5), "condition was not met in time");
            await Task.Delay(5);
        }
    }

    private static int[] Ids(JsonElement array) => array.EnumerateArray().Select(e => e.GetProperty("id").GetInt32()).ToArray();

    // ---- input validation -------------------------------------------------

    [Fact]
    public async Task Missing_ids_returns_400_problem_details()
    {
        using var app = StartApp();
        var (response, body) = await GetAsync(app.CreateClient(), "/aggregate");

        Assert.Equal(HttpStatusCode.BadRequest, response.StatusCode);
        Assert.Equal("application/problem+json", response.Content.Headers.ContentType?.MediaType);
        Assert.Equal(400, body.GetProperty("status").GetInt32());
    }

    [Theory]
    [InlineData("")]
    [InlineData("1,a")]
    [InlineData("1,,2")]
    [InlineData("-1")]
    [InlineData("1.5")]
    public async Task Empty_or_non_numeric_ids_return_400(string ids)
    {
        using var app = StartApp();
        var (response, _) = await GetAsync(app.CreateClient(), $"/aggregate?ids={ids}");

        Assert.Equal(HttpStatusCode.BadRequest, response.StatusCode);
    }

    [Fact]
    public async Task More_than_50_ids_return_400_and_call_nothing()
    {
        using var app = StartApp();
        var ids = string.Join(',', Enumerable.Range(1, 51));
        var (response, _) = await GetAsync(app.CreateClient(), $"/aggregate?ids={ids}");

        Assert.Equal(HttpStatusCode.BadRequest, response.StatusCode);
        Assert.Equal(0, _upstream.Calls(1));
    }

    [Fact]
    public async Task Exactly_50_ids_are_accepted()
    {
        using var app = StartApp();
        var ids = string.Join(',', Enumerable.Range(1, 50));
        var (response, body) = await GetAsync(app.CreateClient(), $"/aggregate?ids={ids}");

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.Equal(50, body.GetProperty("results").GetArrayLength());
    }

    [Fact]
    public async Task Spaces_around_ids_are_ignored()
    {
        using var app = StartApp();
        var (response, body) = await GetAsync(app.CreateClient(), "/aggregate?ids=1,%202");

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.Equal([1, 2], Ids(body.GetProperty("results")));
    }

    // ---- aggregation ------------------------------------------------------

    [Fact]
    public async Task Upstream_json_is_wrapped_in_data()
    {
        using var app = StartApp();
        var (response, body) = await GetAsync(app.CreateClient(), "/aggregate?ids=7");

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        var item = body.GetProperty("results")[0];
        Assert.Equal(7, item.GetProperty("id").GetInt32());
        Assert.Equal("item-7", item.GetProperty("data").GetProperty("name").GetString());
        Assert.Equal(0, body.GetProperty("errors").GetArrayLength());
    }

    [Fact]
    public async Task Results_keep_request_order_even_when_upstream_answers_out_of_order()
    {
        _upstream.Script = (id, _) => new Behavior(DelayMs: id == 1 ? 120 : id == 2 ? 60 : 0);
        using var app = StartApp();
        var (_, body) = await GetAsync(app.CreateClient(), "/aggregate?ids=1,2,3");

        Assert.Equal([1, 2, 3], Ids(body.GetProperty("results")));
    }

    [Fact]
    public async Task Partial_failure_returns_200_with_results_and_errors_in_order()
    {
        _upstream.Script = (id, _) => id is 2 or 4 ? new Behavior(Status: 404) : new Behavior();
        using var app = StartApp();
        var (response, body) = await GetAsync(app.CreateClient(), "/aggregate?ids=1,2,3,4");

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.Equal([1, 3], Ids(body.GetProperty("results")));
        Assert.Equal([2, 4], Ids(body.GetProperty("errors")));
        Assert.Equal("upstream status 404", body.GetProperty("errors")[0].GetProperty("error").GetString());
    }

    // ---- concurrency ------------------------------------------------------

    [Fact]
    public async Task Never_has_more_than_max_concurrency_calls_in_flight()
    {
        _upstream.Script = (_, _) => new Behavior(DelayMs: 40);
        using var app = StartApp(o => o.MaxConcurrency = 2);
        var (response, body) = await GetAsync(app.CreateClient(), "/aggregate?ids=1,2,3,4,5,6,7,8");

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.Equal(8, body.GetProperty("results").GetArrayLength());
        Assert.Equal(2, _upstream.MaxInFlight);
    }

    [Fact]
    public async Task Default_max_concurrency_is_4_and_calls_really_overlap()
    {
        _upstream.Script = (_, _) => new Behavior(DelayMs: 40);
        using var app = StartApp();
        await GetAsync(app.CreateClient(), "/aggregate?ids=1,2,3,4,5,6,7,8,9,10");

        Assert.Equal(4, _upstream.MaxInFlight);
    }

    // ---- retries and timeouts --------------------------------------------

    [Fact]
    public async Task Retries_a_5xx_and_succeeds_on_the_third_attempt()
    {
        _upstream.Script = (_, call) => call < 3 ? new Behavior(Status: 503) : new Behavior();
        using var app = StartApp();
        var (_, body) = await GetAsync(app.CreateClient(), "/aggregate?ids=5");

        Assert.Equal([5], Ids(body.GetProperty("results")));
        Assert.Equal(3, _upstream.Calls(5));
    }

    [Fact]
    public async Task Gives_up_after_two_retries_and_reports_the_status()
    {
        _upstream.Script = (id, _) => id == 3 ? new Behavior(Status: 500) : new Behavior();
        using var app = StartApp();
        var (response, body) = await GetAsync(app.CreateClient(), "/aggregate?ids=1,3");

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.Equal(3, _upstream.Calls(3));
        Assert.Equal(1, _upstream.Calls(1));
        Assert.Equal([1], Ids(body.GetProperty("results")));
        Assert.Equal("upstream status 500", body.GetProperty("errors")[0].GetProperty("error").GetString());
    }

    [Fact]
    public async Task Retries_follow_max_retries_setting()
    {
        _upstream.Script = (_, _) => new Behavior(Status: 500);
        using var app = StartApp(o => o.MaxRetries = 0);
        await GetAsync(app.CreateClient(), "/aggregate?ids=1");

        Assert.Equal(1, _upstream.Calls(1));
    }

    [Fact]
    public async Task Does_not_retry_4xx()
    {
        _upstream.Script = (_, _) => new Behavior(Status: 404);
        using var app = StartApp();
        var (_, body) = await GetAsync(app.CreateClient(), "/aggregate?ids=9");

        Assert.Equal(1, _upstream.Calls(9));
        Assert.Equal("upstream status 404", body.GetProperty("errors")[0].GetProperty("error").GetString());
    }

    [Fact]
    public async Task Retries_dropped_connections_then_recovers()
    {
        _upstream.Script = (_, call) => call == 1 ? new Behavior(Abort: true) : new Behavior();
        using var app = StartApp();
        var (_, body) = await GetAsync(app.CreateClient(), "/aggregate?ids=2");

        Assert.Equal([2], Ids(body.GetProperty("results")));
        Assert.Equal(2, _upstream.Calls(2));
    }

    [Fact]
    public async Task Reports_unreachable_after_repeated_network_errors()
    {
        _upstream.Script = (_, _) => new Behavior(Abort: true);
        using var app = StartApp();
        var (_, body) = await GetAsync(app.CreateClient(), "/aggregate?ids=2");

        Assert.Equal(3, _upstream.Calls(2));
        Assert.Equal("upstream unreachable", body.GetProperty("errors")[0].GetProperty("error").GetString());
    }

    [Fact]
    public async Task Per_call_timeout_is_retried_and_then_reported_as_timeout()
    {
        _upstream.Script = (_, _) => new Behavior(DelayMs: 10_000);
        using var app = StartApp(o => o.CallTimeoutMs = 50);
        var (response, body) = await GetAsync(app.CreateClient(), "/aggregate?ids=4");

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.Equal(3, _upstream.Calls(4));
        Assert.Equal("timeout", body.GetProperty("errors")[0].GetProperty("error").GetString());
        await Eventually(() => _upstream.Cancelled == 3);
    }

    // ---- cancellation -----------------------------------------------------

    [Fact]
    public async Task Overall_deadline_cancels_outstanding_upstream_calls()
    {
        _upstream.Script = (id, _) => id == 2 ? new Behavior(DelayMs: 10_000) : new Behavior();
        using var app = StartApp(o =>
        {
            o.OverallTimeoutMs = 150;
            o.CallTimeoutMs = 60_000;
        });
        var (response, body) = await GetAsync(app.CreateClient(), "/aggregate?ids=1,2");

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.Equal([1], Ids(body.GetProperty("results")));
        Assert.Equal([2], Ids(body.GetProperty("errors")));
        Assert.Equal("deadline exceeded", body.GetProperty("errors")[0].GetProperty("error").GetString());
        await Eventually(() => _upstream.Cancelled == 1);
    }

    [Fact]
    public async Task Client_disconnect_cancels_outstanding_upstream_calls()
    {
        _upstream.Script = (_, _) => new Behavior(DelayMs: 10_000);
        using var app = StartApp(o => o.CallTimeoutMs = 60_000);
        var client = app.CreateClient();
        using var cts = new CancellationTokenSource();

        var request = client.GetAsync("/aggregate?ids=1,2", cts.Token);
        await Eventually(() => _upstream.InFlight == 2);
        await cts.CancelAsync();

        await Assert.ThrowsAnyAsync<OperationCanceledException>(() => request);
        await Eventually(() => _upstream.Cancelled == 2);
    }
}
