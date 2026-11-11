using System.Net;
using System.Text;

namespace M03.Exercises;

public class Ex3_IdempotencyTests
{
    static Task<StoredResponse> Ok(string body) => Task.FromResult(new StoredResponse(201, body, "/x/1"));

    [Fact]
    public async Task FirstCallRunsTheAction()
    {
        var store = new IdempotencyStore();

        var result = await store.ExecuteAsync("k1", "body-a", () => Ok("first"));

        Assert.Equal(IdempotencyOutcome.Executed, result.Outcome);
        Assert.Equal("first", result.Response?.Body);
    }

    [Fact]
    public async Task SameKeyAndFingerprintReplaysWithoutRunningAgain()
    {
        var store = new IdempotencyStore();
        var runs = 0;
        Task<StoredResponse> Action()
        {
            runs++;
            return Ok($"run {runs}");
        }

        await store.ExecuteAsync("k1", "body-a", Action);
        var second = await store.ExecuteAsync("k1", "body-a", Action);

        Assert.Equal(IdempotencyOutcome.Replayed, second.Outcome);
        Assert.Equal("run 1", second.Response?.Body);
        Assert.Equal(1, runs);
    }

    [Fact]
    public async Task SameKeyWithAnotherFingerprintIsAMismatch()
    {
        var store = new IdempotencyStore();
        var runs = 0;
        Task<StoredResponse> Action()
        {
            runs++;
            return Ok("x");
        }

        await store.ExecuteAsync("k1", "body-a", Action);
        var second = await store.ExecuteAsync("k1", "body-b", Action);

        Assert.Equal(IdempotencyOutcome.Mismatch, second.Outcome);
        Assert.Null(second.Response);
        Assert.Equal(1, runs);
    }

    [Fact]
    public async Task DifferentKeysAreIndependent()
    {
        var store = new IdempotencyStore();

        var a = await store.ExecuteAsync("k1", "same", () => Ok("a"));
        var b = await store.ExecuteAsync("k2", "same", () => Ok("b"));

        Assert.Equal(IdempotencyOutcome.Executed, a.Outcome);
        Assert.Equal(IdempotencyOutcome.Executed, b.Outcome);
        Assert.Equal("b", b.Response?.Body);
    }

    [Fact]
    public async Task AFailedActionIsNotRememberedAndCanBeRetried()
    {
        var store = new IdempotencyStore();

        await Assert.ThrowsAsync<InvalidOperationException>(() =>
            store.ExecuteAsync("k1", "body", () => throw new InvalidOperationException("db down")));
        var retry = await store.ExecuteAsync("k1", "body", () => Ok("recovered"));

        Assert.Equal(IdempotencyOutcome.Executed, retry.Outcome);
        Assert.Equal("recovered", retry.Response?.Body);
    }

    [Fact]
    public async Task ConcurrentRetriesRunTheActionOnce()
    {
        var store = new IdempotencyStore();
        var gate = new TaskCompletionSource();
        var runs = 0;

        async Task<StoredResponse> Slow()
        {
            Interlocked.Increment(ref runs);
            await gate.Task;
            return new StoredResponse(201, "once", "/x/1");
        }

        var first = store.ExecuteAsync("k1", "body", Slow);
        var second = store.ExecuteAsync("k1", "body", Slow);
        var third = store.ExecuteAsync("k1", "body", Slow);
        await Task.Delay(20);
        Assert.False(first.IsCompleted);
        Assert.False(second.IsCompleted);

        gate.SetResult();
        var results = await Task.WhenAll(first, second, third);

        Assert.Equal(1, runs);
        Assert.All(results, r => Assert.Equal("once", r.Response?.Body));
        Assert.Equal(1, results.Count(r => r.Outcome == IdempotencyOutcome.Executed));
        Assert.Equal(2, results.Count(r => r.Outcome == IdempotencyOutcome.Replayed));
    }

    static StringContent Json(string json) => new(json, Encoding.UTF8, "application/json");

    static async Task<(TestApp App, Func<int> Created)> StartAsync()
    {
        var created = 0;
        var app = await TestApp.StartAsync(a => PaymentEndpoints.MapCreateTask(a, new IdempotencyStore(), () => Interlocked.Increment(ref created)));
        return (app, () => created);
    }

    static HttpRequestMessage Post(string body, string? key)
    {
        var request = new HttpRequestMessage(HttpMethod.Post, "/tasks") { Content = Json(body) };
        if (key is not null)
        {
            request.Headers.Add("Idempotency-Key", key);
        }

        return request;
    }

    [Fact]
    public async Task RetryWithTheSameKeyGetsTheOriginalResponse()
    {
        var (app, created) = await StartAsync();
        await using var _ = app;

        var first = await app.Client.SendAsync(Post("""{"title":"a"}""", "abc"));
        var retry = await app.Client.SendAsync(Post("""{"title":"a"}""", "abc"));

        Assert.Equal(HttpStatusCode.Created, retry.StatusCode);
        Assert.Equal(first.Headers.Location, retry.Headers.Location);
        Assert.False(first.Headers.Contains("Idempotent-Replayed"));
        Assert.Equal("true", Assert.Single(retry.Headers.GetValues("Idempotent-Replayed")));
        Assert.Equal(1, created());
    }

    [Fact]
    public async Task ReusingAKeyWithAnotherBodyIs422()
    {
        var (app, created) = await StartAsync();
        await using var _ = app;

        await app.Client.SendAsync(Post("""{"title":"a"}""", "abc"));
        var other = await app.Client.SendAsync(Post("""{"title":"b"}""", "abc"));

        Assert.Equal(HttpStatusCode.UnprocessableEntity, other.StatusCode);
        Assert.Equal(1, created());
    }

    [Fact]
    public async Task WithoutAKeyEveryRequestCreatesSomething()
    {
        var (app, created) = await StartAsync();
        await using var _ = app;

        var first = await app.Client.SendAsync(Post("""{"title":"a"}""", null));
        var second = await app.Client.SendAsync(Post("""{"title":"a"}""", null));

        Assert.NotEqual(first.Headers.Location, second.Headers.Location);
        Assert.Equal(2, created());
    }
}
