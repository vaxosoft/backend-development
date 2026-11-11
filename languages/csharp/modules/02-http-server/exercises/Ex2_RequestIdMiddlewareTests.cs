using System.Net;
using System.Text.RegularExpressions;

namespace M02.Exercises;

public class Ex2_RequestIdMiddlewareTests
{
    static Task<TestApp> StartAsync() => TestApp.StartAsync(app =>
    {
        app.UseRequestId();
        app.MapGet("/id", (HttpContext context) => RequestIdMiddleware.Get(context));
        app.MapGet("/teapot", () => Results.StatusCode(StatusCodes.Status418ImATeapot));
    });

    static string HeaderOf(HttpResponseMessage response) =>
        Assert.Single(response.Headers.GetValues(RequestIdMiddleware.HeaderName));

    [Fact]
    public async Task GeneratesA32CharHexIdWhenNoneIsSent()
    {
        await using var app = await StartAsync();
        var response = await app.Client.GetAsync("/id");

        Assert.Matches(new Regex("^[0-9a-f]{32}$"), HeaderOf(response));
    }

    [Fact]
    public async Task GeneratedIdsAreUniquePerRequest()
    {
        await using var app = await StartAsync();
        var first = HeaderOf(await app.Client.GetAsync("/id"));
        var second = HeaderOf(await app.Client.GetAsync("/id"));

        Assert.NotEqual(first, second);
    }

    [Fact]
    public async Task EchoesAnAcceptableIncomingId()
    {
        await using var app = await StartAsync();
        using var request = new HttpRequestMessage(HttpMethod.Get, "/id");
        request.Headers.Add(RequestIdMiddleware.HeaderName, "client-id_1.2");

        var response = await app.Client.SendAsync(request);

        Assert.Equal("client-id_1.2", HeaderOf(response));
    }

    [Fact]
    public async Task HandlersSeeTheSameIdAsTheResponseHeader()
    {
        await using var app = await StartAsync();
        var response = await app.Client.GetAsync("/id");

        Assert.Equal(HeaderOf(response), await response.Content.ReadAsStringAsync());
    }

    [Theory]
    [InlineData("has space")]
    [InlineData("semi;colon")]
    public async Task ReplacesUnsafeIds(string unsafeId)
    {
        await using var app = await StartAsync();
        using var request = new HttpRequestMessage(HttpMethod.Get, "/id");
        request.Headers.TryAddWithoutValidation(RequestIdMiddleware.HeaderName, unsafeId);

        var response = await app.Client.SendAsync(request);

        Assert.Matches(new Regex("^[0-9a-f]{32}$"), HeaderOf(response));
    }

    [Fact]
    public async Task ReplacesOverlongIds()
    {
        await using var app = await StartAsync();
        using var request = new HttpRequestMessage(HttpMethod.Get, "/id");
        request.Headers.Add(RequestIdMiddleware.HeaderName, new string('a', 65));

        var response = await app.Client.SendAsync(request);

        Assert.Equal(32, HeaderOf(response).Length);
    }

    [Fact]
    public async Task ErrorAndNotFoundResponsesCarryTheHeaderToo()
    {
        await using var app = await StartAsync();

        var teapot = await app.Client.GetAsync("/teapot");
        var missing = await app.Client.GetAsync("/missing");

        Assert.Equal((HttpStatusCode)418, teapot.StatusCode);
        Assert.Equal(32, HeaderOf(teapot).Length);
        Assert.Equal(HttpStatusCode.NotFound, missing.StatusCode);
        Assert.Equal(32, HeaderOf(missing).Length);
    }
}
