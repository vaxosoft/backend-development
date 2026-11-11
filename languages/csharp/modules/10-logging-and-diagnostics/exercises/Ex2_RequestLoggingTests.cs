using Microsoft.AspNetCore.Http;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Time.Testing;

namespace M10.Exercises;

public class Ex2_RequestLoggingTests
{
    private const string Template = "HTTP {Method} {Path} responded {StatusCode} in {ElapsedMs} ms";

    private static (RequestLoggingMiddleware Middleware, ListLogger<RequestLoggingMiddleware> Log, FakeTimeProvider Time) Create(Func<HttpContext, FakeTimeProvider, Task> next)
    {
        var time = new FakeTimeProvider();
        var log = new ListLogger<RequestLoggingMiddleware>();
        return (new RequestLoggingMiddleware(ctx => next(ctx, time), log, time), log, time);
    }

    [Fact]
    public async Task Logs_one_structured_line_with_method_path_status_and_elapsed_time()
    {
        var (middleware, log, _) = Create((ctx, time) =>
        {
            time.Advance(TimeSpan.FromMilliseconds(250));
            ctx.Response.StatusCode = 201;
            return Task.CompletedTask;
        });
        var context = HttpTestExtensions.NewContext("POST", "/tasks");
        context.Request.QueryString = new QueryString("?token=secret");

        await middleware.InvokeAsync(context);

        var entry = Assert.Single(log.Records);
        Assert.Equal(LogLevel.Information, entry.Level);
        Assert.Equal(Template, entry.Template);
        Assert.Equal("POST", entry.Properties["Method"]);
        Assert.Equal("/tasks", entry.Properties["Path"]);
        Assert.Equal(201, entry.Properties["StatusCode"]);
        Assert.Equal(250.0, Assert.IsType<double>(entry.Properties["ElapsedMs"]), precision: 3);
        Assert.DoesNotContain("secret", entry.Message);
    }

    [Theory]
    [InlineData(200, LogLevel.Information)]
    [InlineData(302, LogLevel.Information)]
    [InlineData(404, LogLevel.Warning)]
    [InlineData(422, LogLevel.Warning)]
    [InlineData(503, LogLevel.Error)]
    public async Task The_level_follows_the_status_class(int status, LogLevel expected)
    {
        var (middleware, log, _) = Create((ctx, _) =>
        {
            ctx.Response.StatusCode = status;
            return Task.CompletedTask;
        });

        await middleware.InvokeAsync(HttpTestExtensions.NewContext());

        Assert.Equal(expected, Assert.Single(log.Records).Level);
    }

    [Fact]
    public async Task An_incoming_request_id_is_echoed_and_scoped_around_downstream_logging()
    {
        ListLogger<RequestLoggingMiddleware>? log = null;
        var (middleware, logger, _) = Create((_, _) =>
        {
            log!.Log(LogLevel.Information, new EventId(0), "inside handler", null, (s, _) => s);
            return Task.CompletedTask;
        });
        log = logger;
        var context = HttpTestExtensions.NewContext();
        context.Request.Headers["X-Request-ID"] = "req-77";

        await middleware.InvokeAsync(context);

        Assert.Equal("req-77", context.Response.Headers["X-Request-ID"].ToString());
        Assert.Equal(2, log.Records.Count);
        Assert.Equal("req-77", log.Records[0].Scope["RequestId"]);
        Assert.Equal("req-77", log.Records[1].Scope["RequestId"]);
    }

    [Fact]
    public async Task A_missing_request_id_is_generated()
    {
        var (middleware, log, _) = Create((_, _) => Task.CompletedTask);
        var context = HttpTestExtensions.NewContext();

        await middleware.InvokeAsync(context);

        var id = context.Response.Headers["X-Request-ID"].ToString();
        Assert.Matches("^[0-9a-f]{32}$", id);
        Assert.Equal(id, Assert.Single(log.Records).Scope["RequestId"]);
    }

    [Fact]
    public async Task The_scope_is_closed_after_the_request()
    {
        var (middleware, log, _) = Create((_, _) => Task.CompletedTask);

        await middleware.InvokeAsync(HttpTestExtensions.NewContext());
        log.Log(LogLevel.Information, new EventId(0), "later", null, (s, _) => s);

        Assert.Empty(log.Records[1].Scope);
    }

    [Fact]
    public async Task An_exception_is_logged_as_a_500_and_rethrown()
    {
        var boom = new InvalidOperationException("boom");
        var (middleware, log, _) = Create((_, _) => throw boom);

        var thrown = await Assert.ThrowsAsync<InvalidOperationException>(() => middleware.InvokeAsync(HttpTestExtensions.NewContext("GET", "/boom")));

        Assert.Same(boom, thrown);
        var entry = Assert.Single(log.Records);
        Assert.Equal(LogLevel.Error, entry.Level);
        Assert.Equal(500, entry.Properties["StatusCode"]);
        Assert.Same(boom, entry.Exception);
    }
}
