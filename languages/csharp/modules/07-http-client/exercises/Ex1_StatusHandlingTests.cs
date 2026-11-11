using System.Net;
using static M07.Exercises.ScriptedHandler;

namespace M07.Exercises;

public class Ex1_StatusHandlingTests
{
    private static (TaskTitleClient Client, ScriptedHandler Handler) Create(params Func<HttpRequestMessage, CancellationToken, Task<HttpResponseMessage>>[] script)
    {
        var handler = new ScriptedHandler(script);
        return (new TaskTitleClient(new HttpClient(handler) { BaseAddress = new Uri("http://tracker.test/api/") }), handler);
    }

    [Fact]
    public async Task Returns_the_title_of_an_existing_task()
    {
        var (client, handler) = Create(Status(HttpStatusCode.OK, """{"id":7,"title":"Write docs","status":"todo"}"""));

        Assert.Equal("Write docs", await client.GetTitleAsync(7));

        var seen = Assert.Single(handler.Seen);
        Assert.Equal(HttpMethod.Get, seen.Method);
        Assert.Equal("http://tracker.test/api/tasks/7", seen.Uri!.ToString());
        Assert.Contains(seen.Headers.Accept, a => a.MediaType == "application/json");
    }

    [Fact]
    public async Task Not_found_is_an_expected_outcome_not_an_exception()
    {
        var (client, _) = Create(Status(HttpStatusCode.NotFound));

        Assert.Null(await client.GetTitleAsync(99));
    }

    [Theory]
    [InlineData(HttpStatusCode.Unauthorized)]
    [InlineData(HttpStatusCode.UnprocessableEntity)]
    [InlineData(HttpStatusCode.InternalServerError)]
    public async Task Other_failures_surface_as_HttpRequestException_with_the_status(HttpStatusCode status)
    {
        var (client, _) = Create(Status(status));

        var ex = await Assert.ThrowsAsync<HttpRequestException>(() => client.GetTitleAsync(1));
        Assert.Equal(status, ex.StatusCode);
    }

    [Fact]
    public async Task The_response_is_disposed_on_success_and_on_failure()
    {
        var content = new TrackingContent("""{"title":"x"}""");
        var (client, _) = Create((_, _) => Task.FromResult(new HttpResponseMessage(HttpStatusCode.OK) { Content = content }));
        await client.GetTitleAsync(1);
        Assert.True(content.Disposed);

        var failed = new TrackingContent("oops");
        (client, _) = Create((_, _) => Task.FromResult(new HttpResponseMessage(HttpStatusCode.BadGateway) { Content = failed }));
        await Assert.ThrowsAsync<HttpRequestException>(() => client.GetTitleAsync(1));
        Assert.True(failed.Disposed);
    }

    [Fact]
    public async Task Cancellation_reaches_the_transport()
    {
        var (client, _) = Create(Hang());
        using var cts = new CancellationTokenSource();
        var pending = client.GetTitleAsync(1, cts.Token);
        await cts.CancelAsync();

        await Assert.ThrowsAnyAsync<OperationCanceledException>(() => pending);
    }
}
