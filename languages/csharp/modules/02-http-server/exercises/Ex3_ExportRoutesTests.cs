using System.Net;
using System.Text.Json;

namespace M02.Exercises;

public class Ex3_ExportRoutesTests
{
    static async Task<HttpResponseMessage> GetAsync(string? accept)
    {
        await using var app = await TestApp.StartAsync(a => ExportRoutes.Map(a));
        using var request = new HttpRequestMessage(HttpMethod.Get, "/tasks/export");
        if (accept is not null)
        {
            request.Headers.TryAddWithoutValidation("Accept", accept);
        }

        var response = await app.Client.SendAsync(request);
        await response.Content.LoadIntoBufferAsync();
        return response;
    }

    [Fact]
    public async Task DefaultsToJsonWithoutAcceptHeader()
    {
        var response = await GetAsync(null);

        Assert.Equal("application/json", response.Content.Headers.ContentType?.MediaType);
        using var json = JsonDocument.Parse(await response.Content.ReadAsStringAsync());
        Assert.Equal(2, json.RootElement.GetArrayLength());
        Assert.Equal("Write docs", json.RootElement[0].GetProperty("title").GetString());
        Assert.Equal("in_progress", json.RootElement[1].GetProperty("status").GetString());
    }

    [Fact]
    public async Task WildcardGetsJson()
    {
        var response = await GetAsync("*/*");

        Assert.Equal("application/json", response.Content.Headers.ContentType?.MediaType);
    }

    [Fact]
    public async Task ServesCsv()
    {
        var response = await GetAsync("text/csv");

        Assert.Equal("text/csv", response.Content.Headers.ContentType?.MediaType);
        Assert.Equal("id,title,status\n1,Write docs,todo\n2,Fix login,in_progress\n", await response.Content.ReadAsStringAsync());
    }

    [Fact]
    public async Task ServesPlainText()
    {
        var response = await GetAsync("text/plain");

        Assert.Equal("text/plain", response.Content.Headers.ContentType?.MediaType);
        Assert.Equal("1. Write docs [todo]\n2. Fix login [in_progress]\n", await response.Content.ReadAsStringAsync());
    }

    [Theory]
    [InlineData("text/plain;q=0.5, text/csv", "text/csv")]
    [InlineData("application/json;q=0.1, text/csv;q=0.9", "text/csv")]
    [InlineData("*/*;q=0.1, text/plain", "text/plain")]
    [InlineData("text/*", "text/csv")]
    [InlineData("text/*;q=0.8, application/json;q=0.7", "text/csv")]
    [InlineData("application/json;q=0, */*", "text/csv")]
    [InlineData("text/csv;q=0, text/plain;q=0, */*;q=0.5", "application/json")]
    [InlineData("text/html, application/xml;q=0.9, text/plain;q=0.2", "text/plain")]
    public async Task HonoursQualityAndSpecificity(string accept, string expected)
    {
        var response = await GetAsync(accept);

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.Equal(expected, response.Content.Headers.ContentType?.MediaType);
    }

    [Theory]
    [InlineData("application/xml")]
    [InlineData("text/html, image/*")]
    [InlineData("*/*;q=0")]
    public async Task ReturnsNotAcceptableWhenNothingMatches(string accept)
    {
        var response = await GetAsync(accept);

        Assert.Equal(HttpStatusCode.NotAcceptable, response.StatusCode);
    }

    [Theory]
    [InlineData(null)]
    [InlineData("text/csv")]
    [InlineData("application/xml")]
    public async Task AlwaysVariesOnAccept(string? accept)
    {
        var response = await GetAsync(accept);

        Assert.Contains("Accept", response.Headers.Vary);
    }
}
