using System.Collections.Concurrent;
using System.Net;
using System.Net.Http.Headers;
using System.Net.Http.Json;
using System.Text.Json;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Mvc.Testing;
using Microsoft.Data.Sqlite;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.DependencyInjection.Extensions;
using Microsoft.Extensions.Time.Testing;

namespace P5.UrlShortener.Tests;

public sealed class ListSink : ILogLineSink
{
    private readonly ConcurrentQueue<string> _lines = new();

    public IReadOnlyList<string> Lines => [.. _lines];

    public void Write(string line) => _lines.Enqueue(line);
}

/// <summary>One isolated app per test: private in-memory SQLite database, fake clock, captured log lines.</summary>
public sealed class ShortenerApp : WebApplicationFactory<Program>
{
    public const string Password = "correct horse battery";

    private readonly Action<ShortenerOptions>? _configure;
    private readonly string _connectionString = $"Data Source=file:p5-{Guid.NewGuid():N}?mode=memory&cache=shared";

    public ShortenerApp(Action<ShortenerOptions>? configure = null) => _configure = configure;

    public FakeTimeProvider Clock { get; } = new();

    public ListSink Log { get; } = new();

    protected override void ConfigureWebHost(IWebHostBuilder builder) =>
        builder.ConfigureServices(services =>
        {
            services.PostConfigure<ShortenerOptions>(o =>
            {
                o.ConnectionString = _connectionString;
                o.JwtSecret = "test-secret-test-secret-test-secret-123";
                o.Pbkdf2Iterations = 1_000;
                o.RateLimitBurst = 1_000;
                _configure?.Invoke(o);
            });
            services.RemoveAll<TimeProvider>();
            services.AddSingleton<TimeProvider>(Clock);
            services.RemoveAll<ILogLineSink>();
            services.AddSingleton<ILogLineSink>(Log);
        });

    public HttpClient NewClient() => CreateClient(new WebApplicationFactoryClientOptions { AllowAutoRedirect = false });

    /// <summary>Registers (if needed) and logs in; returns an authenticated client.</summary>
    public async Task<HttpClient> LoginAsync(string email)
    {
        var client = NewClient();
        await client.PostAsJsonAsync("/auth/register", new { email, password = Password });
        var login = await client.PostAsJsonAsync("/auth/login", new { email, password = Password });
        Assert.Equal(HttpStatusCode.OK, login.StatusCode);
        var token = (await login.Content.ReadFromJsonAsync<JsonElement>()).GetProperty("token").GetString();
        client.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", token);
        return client;
    }

    /// <summary>Reads a value straight from the database (white-box, for the "stored hashed" test).</summary>
    public string QueryScalar(string sql)
    {
        using var connection = new SqliteConnection(_connectionString);
        connection.Open();
        using var command = connection.CreateCommand();
        command.CommandText = sql;
        return (string)command.ExecuteScalar()!;
    }
}
