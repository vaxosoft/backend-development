using System.Text.Json;
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
    public async Task<AggregateResponse> AggregateAsync(IReadOnlyList<int> ids, CancellationToken cancellationToken)
    {
        using var deadline = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
        deadline.CancelAfter(_options.OverallTimeoutMs);
        using var gate = new SemaphoreSlim(_options.MaxConcurrency);

        var outcomes = await Task.WhenAll(ids.Select(id => FetchAsync(id, gate, deadline.Token)));
        cancellationToken.ThrowIfCancellationRequested();

        var results = new List<ItemResult>();
        var errors = new List<ItemError>();
        foreach (var (id, data, error) in outcomes)
        {
            if (error is null)
            {
                results.Add(new ItemResult(id, data));
            }
            else
            {
                errors.Add(new ItemError(id, error));
            }
        }

        return new AggregateResponse(results, errors);
    }

    private async Task<(int Id, JsonElement Data, string? Error)> FetchAsync(int id, SemaphoreSlim gate, CancellationToken token)
    {
        try
        {
            await gate.WaitAsync(token);
            try
            {
                return (id, await upstream.GetItemAsync(id, token), null);
            }
            finally
            {
                gate.Release();
            }
        }
        catch (UpstreamException e)
        {
            return (id, default, e.Message);
        }
        catch (OperationCanceledException)
        {
            return (id, default, "deadline exceeded");
        }
    }
}
