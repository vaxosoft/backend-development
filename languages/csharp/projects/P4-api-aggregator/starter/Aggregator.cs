using Microsoft.Extensions.Options;

namespace P4.ApiAggregator;

/// <summary>Fans a list of ids out to the upstream, concurrently but bounded.</summary>
public sealed class Aggregator(UpstreamClient upstream, IOptions<AggregatorOptions> options)
{
    private readonly AggregatorOptions _options = options.Value;

    /// <summary>
    /// Fetches every id with at most <see cref="AggregatorOptions.MaxConcurrency"/> calls in flight.
    /// Results and errors keep the order of <paramref name="ids"/>. A failed id goes to <c>Errors</c>
    /// (with the <see cref="UpstreamException"/> message) and never fails the others.
    /// When <see cref="AggregatorOptions.OverallTimeoutMs"/> expires, outstanding calls are cancelled and
    /// their ids are reported as <c>deadline exceeded</c>. When <paramref name="cancellationToken"/>
    /// (the client connection) is cancelled, outstanding calls are cancelled and the cancellation propagates.
    /// </summary>
    public Task<AggregateResponse> AggregateAsync(IReadOnlyList<int> ids, CancellationToken cancellationToken)
    {
        // TODO(P4): bounded fan-out over `upstream.GetItemAsync`.
        _ = (upstream, _options, ids, cancellationToken);
        throw new NotImplementedException("TODO(P4): Aggregator.AggregateAsync");
    }
}
