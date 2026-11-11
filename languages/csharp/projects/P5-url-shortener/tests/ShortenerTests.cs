using System.Net;
using System.Net.Http.Json;
using System.Text.Json;
using System.Text.RegularExpressions;
using Microsoft.AspNetCore.Mvc.Testing;

namespace P5.UrlShortener.Tests;

public sealed class AuthTests
{
    private static JsonElement DecodeJwtPart(string token, int index)
    {
        var part = token.Split('.')[index].Replace('-', '+').Replace('_', '/');
        part = part.PadRight(part.Length + (4 - part.Length % 4) % 4, '=');
        return JsonDocument.Parse(Convert.FromBase64String(part)).RootElement;
    }

    [Fact]
    public async Task Register_returns_201_with_the_new_user()
    {
        using var app = new ShortenerApp();
        var response = await app.NewClient().PostAsJsonAsync("/auth/register", new { email = "Ada@Example.com", password = ShortenerApp.Password });

        Assert.Equal(HttpStatusCode.Created, response.StatusCode);
        var body = await response.Content.ReadFromJsonAsync<JsonElement>();
        Assert.Equal("ada@example.com", body.GetProperty("email").GetString());
        Assert.False(body.TryGetProperty("password", out _));
    }

    [Theory]
    [InlineData("ada@example.com", "short")]
    [InlineData("not-an-email", "long enough password")]
    [InlineData("", "")]
    public async Task Register_rejects_invalid_input_with_422(string email, string password)
    {
        using var app = new ShortenerApp();
        var response = await app.NewClient().PostAsJsonAsync("/auth/register", new { email, password });

        Assert.Equal(HttpStatusCode.UnprocessableEntity, response.StatusCode);
        Assert.Equal("application/problem+json", response.Content.Headers.ContentType?.MediaType);
    }

    [Fact]
    public async Task Register_rejects_a_duplicate_email_with_409()
    {
        using var app = new ShortenerApp();
        var client = app.NewClient();
        await client.PostAsJsonAsync("/auth/register", new { email = "ada@example.com", password = ShortenerApp.Password });
        var again = await client.PostAsJsonAsync("/auth/register", new { email = "ADA@example.com", password = ShortenerApp.Password });

        Assert.Equal(HttpStatusCode.Conflict, again.StatusCode);
    }

    [Fact]
    public async Task Password_is_stored_hashed_not_in_plain_text()
    {
        using var app = new ShortenerApp();
        await app.NewClient().PostAsJsonAsync("/auth/register", new { email = "ada@example.com", password = ShortenerApp.Password });

        var stored = app.QueryScalar("SELECT password_hash FROM users WHERE email = 'ada@example.com'");

        Assert.DoesNotContain(ShortenerApp.Password, stored);
        Assert.True(stored.Length > 40);
    }

    [Fact]
    public async Task Login_returns_an_HS256_jwt_that_expires_after_one_hour()
    {
        using var app = new ShortenerApp();
        var client = app.NewClient();
        await client.PostAsJsonAsync("/auth/register", new { email = "ada@example.com", password = ShortenerApp.Password });
        var response = await client.PostAsJsonAsync("/auth/login", new { email = "ada@example.com", password = ShortenerApp.Password });

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        var token = (await response.Content.ReadFromJsonAsync<JsonElement>()).GetProperty("token").GetString()!;
        Assert.Equal("HS256", DecodeJwtPart(token, 0).GetProperty("alg").GetString());
        var claims = DecodeJwtPart(token, 1);
        Assert.Equal(3600, claims.GetProperty("exp").GetInt64() - claims.GetProperty("iat").GetInt64());
        Assert.Equal(app.Clock.GetUtcNow().ToUnixTimeSeconds(), claims.GetProperty("iat").GetInt64());
        Assert.Matches(@"^\d+$", claims.GetProperty("sub").GetString());
    }

    [Fact]
    public async Task Unknown_user_and_wrong_password_get_the_same_401()
    {
        using var app = new ShortenerApp();
        var client = app.NewClient();
        await client.PostAsJsonAsync("/auth/register", new { email = "ada@example.com", password = ShortenerApp.Password });

        var wrongPassword = await client.PostAsJsonAsync("/auth/login", new { email = "ada@example.com", password = "not the password" });
        var unknownUser = await client.PostAsJsonAsync("/auth/login", new { email = "nobody@example.com", password = ShortenerApp.Password });

        Assert.Equal(HttpStatusCode.Unauthorized, wrongPassword.StatusCode);
        Assert.Equal(HttpStatusCode.Unauthorized, unknownUser.StatusCode);
        var a = await wrongPassword.Content.ReadFromJsonAsync<JsonElement>();
        var b = await unknownUser.Content.ReadFromJsonAsync<JsonElement>();
        Assert.Equal(a.GetProperty("detail").GetString(), b.GetProperty("detail").GetString());
    }

    [Fact]
    public async Task Token_is_rejected_after_it_expires()
    {
        using var app = new ShortenerApp();
        var client = await app.LoginAsync("ada@example.com");
        Assert.Equal(HttpStatusCode.OK, (await client.GetAsync("/links")).StatusCode);

        app.Clock.Advance(TimeSpan.FromHours(1) + TimeSpan.FromSeconds(1));

        Assert.Equal(HttpStatusCode.Unauthorized, (await client.GetAsync("/links")).StatusCode);
    }

    [Fact]
    public async Task Token_signed_with_another_secret_is_rejected()
    {
        using var other = new ShortenerApp(o => o.JwtSecret = "another-secret-another-secret-12345678");
        var foreign = await other.LoginAsync("ada@example.com");
        using var app = new ShortenerApp();
        var client = app.NewClient();
        client.DefaultRequestHeaders.Authorization = foreign.DefaultRequestHeaders.Authorization;

        Assert.Equal(HttpStatusCode.Unauthorized, (await client.GetAsync("/links")).StatusCode);
    }
}

public sealed class LinkTests
{
    private static async Task<JsonElement> JsonAsync(HttpResponseMessage response) =>
        await response.Content.ReadFromJsonAsync<JsonElement>();

    [Fact]
    public async Task Creating_a_link_requires_authentication()
    {
        using var app = new ShortenerApp();
        var response = await app.NewClient().PostAsJsonAsync("/links", new { url = "https://example.com" });

        Assert.Equal(HttpStatusCode.Unauthorized, response.StatusCode);
    }

    [Fact]
    public async Task Link_without_slug_gets_a_generated_7_character_slug()
    {
        using var app = new ShortenerApp();
        var client = await app.LoginAsync("ada@example.com");
        var response = await client.PostAsJsonAsync("/links", new { url = "https://example.com/docs" });

        Assert.Equal(HttpStatusCode.Created, response.StatusCode);
        var body = await JsonAsync(response);
        Assert.Matches("^[A-Za-z0-9]{7}$", body.GetProperty("slug").GetString());
        Assert.Equal("https://example.com/docs", body.GetProperty("url").GetString());
    }

    [Theory]
    [InlineData("ftp://example.com/file")]
    [InlineData("example.com")]
    [InlineData("/relative/path")]
    [InlineData("javascript:alert(1)")]
    [InlineData("")]
    public async Task Link_url_must_be_an_absolute_http_or_https_url(string url)
    {
        using var app = new ShortenerApp();
        var client = await app.LoginAsync("ada@example.com");
        var response = await client.PostAsJsonAsync("/links", new { url });

        Assert.Equal(HttpStatusCode.UnprocessableEntity, response.StatusCode);
    }

    [Theory]
    [InlineData("ab")]
    [InlineData("has space")]
    [InlineData("a/b/c")]
    [InlineData("0123456789012345678901234567890123")]
    [InlineData("links")]
    public async Task Link_slug_must_be_3_to_32_url_safe_characters(string slug)
    {
        using var app = new ShortenerApp();
        var client = await app.LoginAsync("ada@example.com");
        var response = await client.PostAsJsonAsync("/links", new { url = "https://example.com", slug });

        Assert.Equal(HttpStatusCode.UnprocessableEntity, response.StatusCode);
    }

    [Fact]
    public async Task Custom_slug_is_used_and_a_duplicate_slug_returns_409_even_for_another_user()
    {
        using var app = new ShortenerApp();
        var ada = await app.LoginAsync("ada@example.com");
        var bob = await app.LoginAsync("bob@example.com");

        var first = await ada.PostAsJsonAsync("/links", new { url = "https://example.com/a", slug = "my_slug-1" });
        var second = await bob.PostAsJsonAsync("/links", new { url = "https://example.com/b", slug = "my_slug-1" });

        Assert.Equal(HttpStatusCode.Created, first.StatusCode);
        Assert.Equal("my_slug-1", (await JsonAsync(first)).GetProperty("slug").GetString());
        Assert.Equal(HttpStatusCode.Conflict, second.StatusCode);
    }

    [Fact]
    public async Task Public_slug_redirects_with_302_and_an_unknown_slug_is_404()
    {
        using var app = new ShortenerApp();
        var ada = await app.LoginAsync("ada@example.com");
        await ada.PostAsJsonAsync("/links", new { url = "https://example.com/some/path?q=1", slug = "docs" });

        var response = await app.NewClient().GetAsync("/docs");

        Assert.Equal(HttpStatusCode.Found, response.StatusCode);
        Assert.Equal("https://example.com/some/path?q=1", response.Headers.Location?.ToString());
        Assert.Equal(HttpStatusCode.NotFound, (await app.NewClient().GetAsync("/nope123")).StatusCode);
    }

    [Fact]
    public async Task Every_redirect_increments_the_hit_counter()
    {
        using var app = new ShortenerApp();
        var ada = await app.LoginAsync("ada@example.com");
        await ada.PostAsJsonAsync("/links", new { url = "https://example.com", slug = "hits" });
        var anonymous = app.NewClient();
        await anonymous.GetAsync("/hits");
        await anonymous.GetAsync("/hits");
        await anonymous.GetAsync("/hits");

        var list = await JsonAsync(await ada.GetAsync("/links"));

        Assert.Equal(3, list.GetProperty("items")[0].GetProperty("hits").GetInt32());
    }

    [Fact]
    public async Task Listing_returns_only_the_callers_links()
    {
        using var app = new ShortenerApp();
        var ada = await app.LoginAsync("ada@example.com");
        var bob = await app.LoginAsync("bob@example.com");
        await ada.PostAsJsonAsync("/links", new { url = "https://example.com/1", slug = "ada-one" });
        await ada.PostAsJsonAsync("/links", new { url = "https://example.com/2", slug = "ada-two" });
        await bob.PostAsJsonAsync("/links", new { url = "https://example.com/3", slug = "bob-one" });

        var list = await JsonAsync(await ada.GetAsync("/links"));

        Assert.Equal(2, list.GetProperty("total").GetInt32());
        Assert.Equal(["ada-one", "ada-two"], list.GetProperty("items").EnumerateArray().Select(i => i.GetProperty("slug").GetString()!).ToArray());
        Assert.Equal(0, list.GetProperty("items")[0].GetProperty("hits").GetInt32());
    }

    [Fact]
    public async Task Owner_can_delete_a_link_and_it_stops_redirecting()
    {
        using var app = new ShortenerApp();
        var ada = await app.LoginAsync("ada@example.com");
        await ada.PostAsJsonAsync("/links", new { url = "https://example.com", slug = "gone" });

        var delete = await ada.DeleteAsync("/links/gone");

        Assert.Equal(HttpStatusCode.NoContent, delete.StatusCode);
        Assert.Equal(HttpStatusCode.NotFound, (await app.NewClient().GetAsync("/gone")).StatusCode);
    }

    [Fact]
    public async Task Another_user_cannot_delete_the_link_and_gets_403()
    {
        using var app = new ShortenerApp();
        var ada = await app.LoginAsync("ada@example.com");
        var bob = await app.LoginAsync("bob@example.com");
        await ada.PostAsJsonAsync("/links", new { url = "https://example.com", slug = "mine" });

        var delete = await bob.DeleteAsync("/links/mine");

        Assert.Equal(HttpStatusCode.Forbidden, delete.StatusCode);
        Assert.Equal(HttpStatusCode.Found, (await app.NewClient().GetAsync("/mine")).StatusCode);
    }

    [Fact]
    public async Task Deleting_requires_auth_and_an_unknown_slug_is_404()
    {
        using var app = new ShortenerApp();
        var ada = await app.LoginAsync("ada@example.com");

        Assert.Equal(HttpStatusCode.Unauthorized, (await app.NewClient().DeleteAsync("/links/whatever")).StatusCode);
        Assert.Equal(HttpStatusCode.NotFound, (await ada.DeleteAsync("/links/whatever")).StatusCode);
    }
}

public sealed class RateLimitTests
{
    private static Task<HttpResponseMessage> Create(HttpClient client) =>
        client.PostAsJsonAsync("/links", new { url = "https://example.com" });

    [Fact]
    public async Task Creating_links_beyond_the_burst_returns_429_with_retry_after()
    {
        using var app = new ShortenerApp(o =>
        {
            o.RateLimitBurst = 3;
            o.RateLimitRefillPerSecond = 0.01;
        });
        var ada = await app.LoginAsync("ada@example.com");

        for (var i = 0; i < 3; i++)
        {
            Assert.Equal(HttpStatusCode.Created, (await Create(ada)).StatusCode);
        }

        var limited = await Create(ada);

        Assert.Equal(HttpStatusCode.TooManyRequests, limited.StatusCode);
        Assert.True(int.Parse(limited.Headers.GetValues("Retry-After").Single()) >= 1);
        Assert.Equal(HttpStatusCode.OK, (await ada.GetAsync("/links")).StatusCode);
    }

    [Fact]
    public async Task The_bucket_is_per_user()
    {
        using var app = new ShortenerApp(o =>
        {
            o.RateLimitBurst = 1;
            o.RateLimitRefillPerSecond = 0.01;
        });
        var ada = await app.LoginAsync("ada@example.com");
        var bob = await app.LoginAsync("bob@example.com");
        await Create(ada);

        Assert.Equal(HttpStatusCode.TooManyRequests, (await Create(ada)).StatusCode);
        Assert.Equal(HttpStatusCode.Created, (await Create(bob)).StatusCode);
    }
}

public sealed class LoggingTests
{
    private static JsonElement Parse(string line) => JsonDocument.Parse(line).RootElement;

    [Fact]
    public async Task Each_request_writes_one_json_line_with_the_required_fields()
    {
        using var app = new ShortenerApp();
        await app.NewClient().GetAsync("/nope123?token=abc");

        var line = Assert.Single(app.Log.Lines);
        var json = Parse(line);
        Assert.Equal("GET", json.GetProperty("method").GetString());
        Assert.Equal("/nope123", json.GetProperty("path").GetString());
        Assert.Equal(404, json.GetProperty("status").GetInt32());
        Assert.True(json.GetProperty("duration_ms").GetDouble() >= 0);
        Assert.False(string.IsNullOrEmpty(json.GetProperty("request_id").GetString()));
        Assert.DoesNotContain("token=abc", line);
    }

    [Fact]
    public async Task Request_id_is_echoed_or_generated_and_matches_the_log()
    {
        using var app = new ShortenerApp();
        var client = app.NewClient();
        var echoed = await client.SendAsync(new HttpRequestMessage(HttpMethod.Get, "/nope1") { Headers = { { "X-Request-ID", "req-123" } } });
        var generated = await client.GetAsync("/nope2");

        Assert.Equal("req-123", echoed.Headers.GetValues("X-Request-ID").Single());
        var generatedId = generated.Headers.GetValues("X-Request-ID").Single();
        Assert.False(string.IsNullOrWhiteSpace(generatedId));
        Assert.Equal(["req-123", generatedId], app.Log.Lines.Select(l => Parse(l).GetProperty("request_id").GetString()!).ToArray());
    }

    [Fact]
    public async Task User_id_is_logged_only_for_authenticated_requests()
    {
        using var app = new ShortenerApp();
        var ada = await app.LoginAsync("ada@example.com");
        var userId = Parse(app.Log.Lines.First(l => l.Contains("/auth/login")));
        Assert.False(userId.TryGetProperty("user_id", out _));

        await ada.GetAsync("/links");

        var authenticated = Parse(app.Log.Lines.Last());
        Assert.Equal("/links", authenticated.GetProperty("path").GetString());
        Assert.True(authenticated.GetProperty("user_id").GetInt64() > 0);
    }

    [Fact]
    public async Task Passwords_and_tokens_never_appear_in_the_log()
    {
        using var app = new ShortenerApp();
        var ada = await app.LoginAsync("ada@example.com");
        var token = ada.DefaultRequestHeaders.Authorization!.Parameter!;
        await ada.PostAsJsonAsync("/links", new { url = "https://example.com", slug = "secret-test" });
        await ada.GetAsync("/links");

        Assert.NotEmpty(app.Log.Lines);
        Assert.All(app.Log.Lines, line =>
        {
            Assert.DoesNotContain(ShortenerApp.Password, line);
            Assert.DoesNotContain(token, line);
            Assert.DoesNotMatch(new Regex(@"eyJ[A-Za-z0-9_-]+\."), line);
        });
    }
}
