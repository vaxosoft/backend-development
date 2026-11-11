using System.Net;
using System.Text.Json;

namespace M02.Exercises;

public class Ex1_GreetingRoutesTests
{
    static Task<TestApp> StartAsync() => TestApp.StartAsync(app => GreetingRoutes.Map(app));

    [Fact]
    public async Task HelloWorld()
    {
        await using var app = await StartAsync();
        var response = await app.Client.GetAsync("/hello");

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.StartsWith("text/plain", response.Content.Headers.ContentType?.MediaType);
        Assert.Equal("Hello, world!", await response.Content.ReadAsStringAsync());
    }

    [Fact]
    public async Task HelloUsesTheRouteParameter()
    {
        await using var app = await StartAsync();

        Assert.Equal("Hello, Ada!", await app.Client.GetStringAsync("/hello/Ada"));
        Assert.Equal("Hello, a b!", await app.Client.GetStringAsync("/hello/a%20b"));
    }

    [Fact]
    public async Task AddReturnsJson()
    {
        await using var app = await StartAsync();
        var response = await app.Client.GetAsync("/add?a=2&b=40");

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.Equal("application/json", response.Content.Headers.ContentType?.MediaType);
        using var json = JsonDocument.Parse(await response.Content.ReadAsStringAsync());
        Assert.Equal(42, json.RootElement.GetProperty("sum").GetInt32());
    }

    [Theory]
    [InlineData("/add")]
    [InlineData("/add?a=1")]
    [InlineData("/add?a=1&b=x")]
    [InlineData("/add?a=1.5&b=2")]
    public async Task AddRejectsMissingOrInvalidOperands(string url)
    {
        await using var app = await StartAsync();

        Assert.Equal(HttpStatusCode.BadRequest, (await app.Client.GetAsync(url)).StatusCode);
    }

    [Fact]
    public async Task HeaderEndpointEchoesTheRequestHeader()
    {
        await using var app = await StartAsync();
        using var request = new HttpRequestMessage(HttpMethod.Get, "/headers/X-Client");
        request.Headers.Add("X-Client", "cli/1.0");

        var response = await app.Client.SendAsync(request);

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.Equal("cli/1.0", await response.Content.ReadAsStringAsync());
    }

    [Fact]
    public async Task MissingHeaderIsNotFound()
    {
        await using var app = await StartAsync();

        Assert.Equal(HttpStatusCode.NotFound, (await app.Client.GetAsync("/headers/X-Nope")).StatusCode);
    }

    [Fact]
    public async Task OtherMethodsAreNotAllowed()
    {
        await using var app = await StartAsync();
        var response = await app.Client.PostAsync("/hello", content: null);

        Assert.Equal(HttpStatusCode.MethodNotAllowed, response.StatusCode);
    }
}
