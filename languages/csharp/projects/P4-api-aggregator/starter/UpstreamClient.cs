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
    public Task<JsonElement> GetItemAsync(int id, CancellationToken cancellationToken)
    {
        // TODO(P4): send the request via `http`, honour `_options`, and retry as described above.
        _ = (http, _options, id, cancellationToken);
        throw new NotImplementedException("TODO(P4): UpstreamClient.GetItemAsync");
    }
}
