using System.Net;
using static M07.Exercises.ScriptedHandler;

namespace M07.Exercises;

public class Ex2_RetryHandlerTests
{
    private sealed class Harness
    {
        public List<TimeSpan> Delays { get; } = [];
        public ScriptedHandler Upstream { get; }
        public HttpClient Client { get; }

        public Harness(int maxRetries, params Func<HttpRequestMessage, CancellationToken, Task<HttpResponseMessage>>[] script)
        {
            Upstream = new ScriptedHandler(script);
            var retry = new RetryHandler(maxRetries, (wait, _) =>
            {
                Delays.Add(wait);
                return Task.CompletedTask;
            })
            { InnerHandler = Upstream };
            Client = new HttpClient(retry) { BaseAddress = new Uri("http://upstream.test/") };
        }
    }

    [Fact]
    public async Task A_healthy_call_is_sent_once_without_waiting()
    {
        var h = new Harness(2, Status(HttpStatusCode.OK));

        using var response = await h.Client.GetAsync("x");

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.Equal(1, h.Upstream.Calls);
        Assert.Empty(h.Delays);
    }

    [Fact]
    public async Task Server_errors_are_retried_with_exponential_backoff()
    {
        var h = new Harness(2, Status(HttpStatusCode.ServiceUnavailable), Status(HttpStatusCode.BadGateway), Status(HttpStatusCode.OK));

        using var response = await h.Client.GetAsync("x");

        Assert.Equal(HttpStatusCode.OK, response.StatusCode);
        Assert.Equal(3, h.Upstream.Calls);
        Assert.Equal([TimeSpan.FromMilliseconds(100), TimeSpan.FromMilliseconds(200)], h.Delays);
    }

    [Fact]
    public async Task When_retries_run_out_the_last_response_is_returned()
    {
        var h = new Harness(2, Status(HttpStatusCode.InternalServerError));

        using var response = await h.Client.GetAsync("x");

        Assert.Equal(HttpStatusCode.InternalServerError, response.StatusCode);
        Assert.Equal(3, h.Upstream.Calls);
    }

    [Theory]
    [InlineData(HttpStatusCode.BadRequest)]
    [InlineData(HttpStatusCode.NotFound)]
    public async Task Client_errors_are_never_retried(HttpStatusCode status)
    {
        var h = new Harness(3, Status(status));

        using var response = await h.Client.GetAsync("x");

        Assert.Equal(status, response.StatusCode);
        Assert.Equal(1, h.Upstream.Calls);
    }

    [Fact]
    public async Task Network_errors_are_retried_and_finally_rethrown()
    {
        var recovers = new Harness(2, Throw(new HttpRequestException("reset")), Status(HttpStatusCode.OK));
        using var ok = await recovers.Client.GetAsync("x");
        Assert.Equal(HttpStatusCode.OK, ok.StatusCode);
        Assert.Equal(2, recovers.Upstream.Calls);

        var down = new Harness(2, Throw(new HttpRequestException("refused")));
        var ex = await Assert.ThrowsAsync<HttpRequestException>(() => down.Client.GetAsync("x"));
        Assert.Equal("refused", ex.Message);
        Assert.Equal(3, down.Upstream.Calls);
    }

    [Fact]
    public async Task An_HttpClient_timeout_is_transient_but_caller_cancellation_is_not()
    {
        var timeout = new TaskCanceledException("timed out", new TimeoutException());
        var h = new Harness(1, Throw(timeout), Status(HttpStatusCode.OK));
        using var ok = await h.Client.GetAsync("x");
        Assert.Equal(2, h.Upstream.Calls);

        var cancelled = new Harness(3, Status(HttpStatusCode.ServiceUnavailable));
        using var cts = new CancellationTokenSource();
        await cts.CancelAsync();
        await Assert.ThrowsAnyAsync<OperationCanceledException>(() => cancelled.Client.GetAsync("x", cts.Token));
        Assert.Equal(0, cancelled.Upstream.Calls);
    }

    [Fact]
    public async Task The_request_body_is_resent_on_every_attempt()
    {
        var h = new Harness(2, Status(HttpStatusCode.ServiceUnavailable), Status(HttpStatusCode.ServiceUnavailable), Status(HttpStatusCode.Created));

        using var response = await h.Client.PostAsync("tasks", new OneShotContent("""{"title":"Write docs"}"""));

        Assert.Equal(HttpStatusCode.Created, response.StatusCode);
        Assert.Equal(3, h.Upstream.Seen.Count);
        Assert.All(h.Upstream.Seen, seen => Assert.Equal("""{"title":"Write docs"}""", seen.Body));
    }
}
