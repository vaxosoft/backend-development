using System.Net.Http.Json;
using System.Text.Json;
using Microsoft.Extensions.Options;

namespace P4.ApiAggregator;

/// <summary>Typed HTTP client for the upstream service (<c>GET {base}/items/{id}</c>).</summary>
public sealed class UpstreamClient(HttpClient http, IOptions<AggregatorOptions> options)
{
    private readonly AggregatorOptions _options = options.Value;

    /// <summary>
    /// Fetches one item. Every attempt gets its own <see cref="AggregatorOptions.CallTimeoutMs"/> timeout.
    /// Network errors, timeouts and 5xx responses are retried up to <see cref="AggregatorOptions.MaxRetries"/>
    /// times with exponential backoff; 4xx responses are never retried.
    /// </summary>
    /// <exception cref="UpstreamException">
    /// After the last attempt, with the message <c>upstream status {code}</c>, <c>timeout</c>,
    /// <c>upstream unreachable</c> or <c>invalid upstream response</c> (a body that is not JSON; never retried).
    /// </exception>
    /// <exception cref="OperationCanceledException">When <paramref name="cancellationToken"/> is cancelled.</exception>
    public async Task<JsonElement> GetItemAsync(int id, CancellationToken cancellationToken)
    {
        for (var attempt = 0; ; attempt++)
        {
            UpstreamException failure;
            try
            {
                return await SendOnceAsync(id, cancellationToken);
            }
            catch (UpstreamException e)
            {
                failure = e;
            }
            catch (HttpRequestException)
            {
                failure = new UpstreamException("upstream unreachable", retryable: true);
            }
            catch (OperationCanceledException) when (!cancellationToken.IsCancellationRequested)
            {
                failure = new UpstreamException("timeout", retryable: true);
            }

            if (!failure.Retryable || attempt >= _options.MaxRetries)
            {
                throw failure;
            }

            await Task.Delay(TimeSpan.FromMilliseconds(_options.BackoffBaseMs * Math.Pow(2, attempt)), cancellationToken);
        }
    }

    private async Task<JsonElement> SendOnceAsync(int id, CancellationToken cancellationToken)
    {
        using var attemptTimeout = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        attemptTimeout.CancelAfter(_options.CallTimeoutMs);

        using var response = await http.GetAsync($"items/{id}", attemptTimeout.Token);
        if (!response.IsSuccessStatusCode)
        {
            var status = (int)response.StatusCode;
            throw new UpstreamException($"upstream status {status}", retryable: status >= 500);
        }

        try
        {
            return await response.Content.ReadFromJsonAsync<JsonElement>(attemptTimeout.Token);
        }
        catch (JsonException)
        {
            throw new UpstreamException("invalid upstream response", retryable: false);
        }
    }
}
