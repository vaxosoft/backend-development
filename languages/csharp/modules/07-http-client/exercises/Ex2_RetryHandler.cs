#pragma warning disable CS9113 // starter: the constructor parameters are for your implementation to use

namespace M07.Exercises;

/// <summary>
/// A <see cref="DelegatingHandler"/> that retries transient failures.
/// <list type="bullet">
/// <item>Up to <paramref name="maxRetries"/> retries after the first attempt.</item>
/// <item>Transient = status 5xx, 408 or 429, <see cref="HttpRequestException"/>, or an HttpClient timeout
/// (a <see cref="TaskCanceledException"/> while the caller's token is NOT cancelled). Other 4xx are never retried.</item>
/// <item>Wait <c>100 ms * 2^(retry - 1)</c> through <paramref name="delay"/> before each retry (100 ms, 200 ms, ...).</item>
/// <item>When retries are exhausted return the last response, or rethrow the last exception.</item>
/// <item>Requests with a body must be re-sent with the same body; discard failed responses before retrying.</item>
/// <item>Caller cancellation is never retried.</item>
/// </list>
/// </summary>
public sealed class RetryHandler(int maxRetries, Func<TimeSpan, CancellationToken, Task> delay) : DelegatingHandler
{
    protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
    {
        throw new NotImplementedException();
    }
}
