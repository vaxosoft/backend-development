using Microsoft.AspNetCore.Http;
using Microsoft.Extensions.Logging;

namespace M10.Exercises;

public class Ex3_ProblemExceptionHandlerTests
{
    private static async Task<(HttpContext Context, ListLogger<ProblemExceptionHandler> Log)> HandleAsync(Exception exception, Action<HttpContext>? configure = null)
    {
        var log = new ListLogger<ProblemExceptionHandler>();
        var context = HttpTestExtensions.NewContext("GET", "/tasks/9");
        configure?.Invoke(context);

        var handled = await new ProblemExceptionHandler(log).TryHandleAsync(context, exception, CancellationToken.None);

        Assert.True(handled);
        return (context, log);
    }

    [Fact]
    public async Task Not_found_becomes_a_404_problem_with_the_message_as_detail()
    {
        var (context, log) = await HandleAsync(new NotFoundException("task 9 does not exist"));

        Assert.Equal(404, context.Response.StatusCode);
        Assert.StartsWith("application/problem+json", context.Response.ContentType);
        using var json = context.ReadJsonBody();
        Assert.Equal(404, json.RootElement.GetProperty("status").GetInt32());
        Assert.Equal("Not found", json.RootElement.GetProperty("title").GetString());
        Assert.Equal("task 9 does not exist", json.RootElement.GetProperty("detail").GetString());

        var entry = Assert.Single(log.Records);
        Assert.Equal(LogLevel.Warning, entry.Level);
        Assert.Null(entry.Exception);
    }

    [Fact]
    public async Task Validation_errors_become_a_422_problem_with_the_field_errors()
    {
        var errors = new Dictionary<string, string[]> { ["title"] = ["must be 1-200 characters"], ["tags"] = ["too many", "must be lowercase"] };

        var (context, log) = await HandleAsync(new ValidationFailedException(errors));

        Assert.Equal(422, context.Response.StatusCode);
        using var json = context.ReadJsonBody();
        Assert.Equal("Validation failed", json.RootElement.GetProperty("title").GetString());
        var body = json.RootElement.GetProperty("errors");
        Assert.Equal("must be 1-200 characters", body.GetProperty("title")[0].GetString());
        Assert.Equal(2, body.GetProperty("tags").GetArrayLength());
        Assert.Equal(LogLevel.Warning, Assert.Single(log.Records).Level);
    }

    [Fact]
    public async Task Unexpected_exceptions_become_a_generic_500_and_are_logged_with_the_exception()
    {
        var secret = new InvalidOperationException("Server=db;Password=hunter2 is unreachable");

        var (context, log) = await HandleAsync(secret);

        Assert.Equal(500, context.Response.StatusCode);
        var body = context.ReadBody();
        Assert.DoesNotContain("hunter2", body);
        Assert.DoesNotContain("unreachable", body);
        using var json = System.Text.Json.JsonDocument.Parse(body);
        Assert.Equal("An unexpected error occurred", json.RootElement.GetProperty("title").GetString());
        Assert.False(json.RootElement.TryGetProperty("detail", out _));

        var entry = Assert.Single(log.Records);
        Assert.Equal(LogLevel.Error, entry.Level);
        Assert.Same(secret, entry.Exception);
        Assert.Equal("GET", entry.Properties["Method"]);
        Assert.Equal("/tasks/9", entry.Properties["Path"]);
        Assert.DoesNotContain("/tasks/9", entry.Template);
    }

    [Fact]
    public async Task The_request_id_is_echoed_into_the_problem_when_present()
    {
        var (context, _) = await HandleAsync(new NotFoundException("x"), c => c.Request.Headers["X-Request-ID"] = "req-5");

        using var json = context.ReadJsonBody();
        Assert.Equal("req-5", json.RootElement.GetProperty("request_id").GetString());

        var (without, _) = await HandleAsync(new NotFoundException("x"));
        using var json2 = without.ReadJsonBody();
        Assert.False(json2.RootElement.TryGetProperty("request_id", out _));
    }

    [Fact]
    public async Task A_client_that_went_away_is_not_an_error()
    {
        using var cts = new CancellationTokenSource();
        await cts.CancelAsync();

        var (context, log) = await HandleAsync(new OperationCanceledException(cts.Token), c => c.RequestAborted = cts.Token);

        Assert.Equal(499, context.Response.StatusCode);
        Assert.Equal(0, context.Response.Body.Length);
        Assert.DoesNotContain(log.Records, r => r.Level >= LogLevel.Information);
    }

    [Fact]
    public async Task A_cancellation_that_is_not_the_clients_is_still_a_server_error()
    {
        var (context, log) = await HandleAsync(new OperationCanceledException("internal timeout"));

        Assert.Equal(500, context.Response.StatusCode);
        Assert.Equal(LogLevel.Error, Assert.Single(log.Records).Level);
    }
}
