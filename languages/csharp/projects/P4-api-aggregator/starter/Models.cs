using System.Text.Json;

namespace P4.ApiAggregator;

public sealed record ItemResult(int Id, JsonElement Data);

public sealed record ItemError(int Id, string Error);

/// <summary>Response body of <c>GET /aggregate</c>; both lists follow the order of the requested ids.</summary>
public sealed record AggregateResponse(IReadOnlyList<ItemResult> Results, IReadOnlyList<ItemError> Errors);

/// <summary>A failed upstream call. <see cref="Exception.Message"/> is what the client sees in <c>errors[].error</c>.</summary>
public sealed class UpstreamException(string message, bool retryable) : Exception(message)
{
    public bool Retryable { get; } = retryable;
}
