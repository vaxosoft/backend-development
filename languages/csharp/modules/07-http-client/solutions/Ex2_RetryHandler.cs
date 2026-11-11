using System.Net;

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
    protected override async Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
    {
        if (request.Content is not null)
        {
            await request.Content.LoadIntoBufferAsync(cancellationToken);
        }

        for (var attempt = 0; ; attempt++)
        {
            cancellationToken.ThrowIfCancellationRequested();
            var lastAttempt = attempt == maxRetries;
            try
            {
                var response = await base.SendAsync(request, cancellationToken);
                if (lastAttempt || !IsTransient(response.StatusCode))
                {
                    return response;
                }

                response.Dispose();
            }
            catch (Exception ex) when (!lastAttempt && IsTransient(ex, cancellationToken))
            {
            }

            await delay(TimeSpan.FromMilliseconds(100 * Math.Pow(2, attempt)), cancellationToken);
        }
    }

    private static bool IsTransient(HttpStatusCode status) => (int)status >= 500 || status is HttpStatusCode.RequestTimeout or HttpStatusCode.TooManyRequests;

    private static bool IsTransient(Exception ex, CancellationToken cancellationToken) =>
        ex is HttpRequestException || (ex is TaskCanceledException && !cancellationToken.IsCancellationRequested);
}
