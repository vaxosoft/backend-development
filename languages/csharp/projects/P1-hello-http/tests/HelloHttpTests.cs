using System.Net;
using System.Text.Json;
using Microsoft.AspNetCore.Server.Kestrel.Core;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Options;

namespace P1.HelloHttp.Tests;

public class HelloHttpTests : IClassFixture<HelloFactory>
{
    private readonly HelloFactory _factory;
    private readonly HttpClient _client;

    public HelloHttpTests(HelloFactory factory)
    {
        _factory = factory;
        _client = factory.CreateClient();
    }

    // --- routes ---------------------------------------------------------

    [Fact]
    public async Task Healthz_ReturnsOkJson()
    {
        var response = await _client.GetAsync("/healthz");

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.Equal("application/json", response.Content.Headers.ContentType?.MediaType);
        using var json = JsonDocument.Parse(await response.Content.ReadAsStringAsync());
        Assert.Equal("ok", json.RootElement.GetProperty("status").GetString());
    }

    [Fact]
    public async Task Hello_WithName_GreetsTheName()
    {
        var response = await _client.GetAsync("/hello/Ada");

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.Equal("Hello, Ada!", await response.Content.ReadAsStringAsync());
    }

    [Fact]
    public async Task Hello_WithoutName_GreetsTheWorld()
    {
        var response = await _client.GetAsync("/hello");

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.Equal("Hello, world!", await response.Content.ReadAsStringAsync());
    }

    [Fact]
    public async Task Hello_IsPlainText()
    {
        var response = await _client.GetAsync("/hello/Ada");

        Assert.Equal("text/plain", response.Content.Headers.ContentType?.MediaType);
    }

    [Fact]
    public async Task Hello_DecodesPercentEncodedNames()
    {
        var response = await _client.GetAsync("/hello/Ada%20Lovelace");

        Assert.Equal("Hello, Ada Lovelace!", await response.Content.ReadAsStringAsync());
    }

    [Fact]
    public async Task UnknownRoute_Returns404()
    {
        var response = await _client.GetAsync("/nope");

        Assert.Equal(HttpStatusCode.NotFound, response.StatusCode);
    }

    // --- /slow ------------------------------------------------------------

    [Fact]
    public async Task Slow_WaitsTheRequestedTimeThenSaysDone()
    {
        var response = await _client.GetAsync("/slow?ms=250");

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.Equal("done", await response.Content.ReadAsStringAsync());
        Assert.Contains(TimeSpan.FromMilliseconds(250), _factory.Time.Delays);
    }

    [Fact]
    public async Task Slow_ClampsTheDelayTo2000Ms()
    {
        using var factory = new HelloFactory();
        using var client = factory.CreateClient();

        var response = await client.GetAsync("/slow?ms=999999");

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.Equal([TimeSpan.FromMilliseconds(2000)], factory.Time.Delays);
    }

    [Theory]
    [InlineData("abc")]
    [InlineData("-5")]
    [InlineData("1.5")]
    public async Task Slow_WithInvalidMs_Returns400(string ms)
    {
        var response = await _client.GetAsync($"/slow?ms={ms}");

        Assert.Equal(HttpStatusCode.BadRequest, response.StatusCode);
    }

    // --- /panic and recovery -------------------------------------------

    [Fact]
    public async Task Panic_Returns500ProblemJson()
    {
        var response = await _client.GetAsync("/panic");

        Assert.Equal(HttpStatusCode.InternalServerError, response.StatusCode);
        Assert.Equal("application/problem+json", response.Content.Headers.ContentType?.MediaType);
        using var json = JsonDocument.Parse(await response.Content.ReadAsStringAsync());
        Assert.Equal(500, json.RootElement.GetProperty("status").GetInt32());
    }

    [Fact]
    public async Task Panic_DoesNotLeakExceptionDetails()
    {
        var response = await _client.GetAsync("/panic");

        var body = await response.Content.ReadAsStringAsync();
        Assert.DoesNotContain("Exception", body);
        Assert.DoesNotContain("   at ", body);
    }

    [Fact]
    public async Task Panic_ServerKeepsServing()
    {
        await _client.GetAsync("/panic");
        await _client.GetAsync("/panic");

        var response = await _client.GetAsync("/hello/still-here");

        Assert.Equal("Hello, still-here!", await response.Content.ReadAsStringAsync());
    }

    // --- request id -----------------------------------------------------

    [Fact]
    public async Task RequestId_IsEchoedWhenProvided()
    {
        using var request = new HttpRequestMessage(HttpMethod.Get, "/healthz");
        request.Headers.Add("X-Request-ID", "abc-123");

        var response = await _client.SendAsync(request);

        Assert.Equal("abc-123", Assert.Single(response.Headers.GetValues("X-Request-ID")));
    }

    [Fact]
    public async Task RequestId_IsGeneratedWhenMissingAndDiffersPerRequest()
    {
        var first = await _client.GetAsync("/healthz");
        var second = await _client.GetAsync("/healthz");

        var id1 = Assert.Single(first.Headers.GetValues("X-Request-ID"));
        var id2 = Assert.Single(second.Headers.GetValues("X-Request-ID"));
        Assert.NotEmpty(id1);
        Assert.NotEqual(id1, id2);
    }

    [Theory]
    [InlineData("0123456789012345678901234567890123456789012345678901234567890123456789")]
    [InlineData("bad id with spaces")]
    public async Task RequestId_IsReplacedWhenIncomingValueIsUnreasonable(string incoming)
    {
        using var request = new HttpRequestMessage(HttpMethod.Get, "/healthz");
        request.Headers.TryAddWithoutValidation("X-Request-ID", incoming);

        var response = await _client.SendAsync(request);

        var id = Assert.Single(response.Headers.GetValues("X-Request-ID"));
        Assert.NotEqual(incoming, id);
        Assert.NotEmpty(id);
    }

    [Fact]
    public async Task RequestId_IsPresentOnErrorResponses()
    {
        using var request = new HttpRequestMessage(HttpMethod.Get, "/panic");
        request.Headers.Add("X-Request-ID", "boom-1");

        var response = await _client.SendAsync(request);

        Assert.Equal("boom-1", Assert.Single(response.Headers.GetValues("X-Request-ID")));
    }

    // --- request logging --------------------------------------------------

    [Fact]
    public async Task Logging_WritesOneEntryPerRequestWithMethodPathStatus()
    {
        using var factory = new HelloFactory();
        using var client = factory.CreateClient();

        await client.GetAsync("/hello/Ada?x=1");
        await client.GetAsync("/nope");

        Assert.Collection(
            factory.Sink.Entries,
            e =>
            {
                Assert.Equal("GET", e.Method);
                Assert.Equal("/hello/Ada", e.Path);
                Assert.Equal(200, e.Status);
                Assert.True(e.Duration >= TimeSpan.Zero);
                Assert.NotEmpty(e.RequestId);
            },
            e =>
            {
                Assert.Equal("/nope", e.Path);
                Assert.Equal(404, e.Status);
            });
    }

    [Fact]
    public async Task Logging_RecordsThePanicAs500()
    {
        using var factory = new HelloFactory();
        using var client = factory.CreateClient();

        await client.GetAsync("/panic");

        var entry = Assert.Single(factory.Sink.Entries);
        Assert.Equal("/panic", entry.Path);
        Assert.Equal(500, entry.Status);
    }

    // --- server configuration ---------------------------------------------

    [Fact]
    public void Server_TimeoutsAreConfigured()
    {
        var kestrel = _factory.Services.GetRequiredService<IOptions<KestrelServerOptions>>().Value.Limits;
        var host = _factory.Services.GetRequiredService<IOptions<HostOptions>>().Value;

        Assert.True(kestrel.RequestHeadersTimeout <= TimeSpan.FromSeconds(10), $"RequestHeadersTimeout was {kestrel.RequestHeadersTimeout}");
        Assert.True(kestrel.KeepAliveTimeout <= TimeSpan.FromSeconds(60), $"KeepAliveTimeout was {kestrel.KeepAliveTimeout}");
        Assert.True(host.ShutdownTimeout <= TimeSpan.FromSeconds(15), $"ShutdownTimeout was {host.ShutdownTimeout}");
    }
}
