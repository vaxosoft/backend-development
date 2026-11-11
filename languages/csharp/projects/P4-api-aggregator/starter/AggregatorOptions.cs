namespace P4.ApiAggregator;

/// <summary>Settings bound from the <c>Aggregator</c> configuration section (env: <c>Aggregator__MaxConcurrency</c>, ...).</summary>
public sealed class AggregatorOptions
{
    public const string SectionName = "Aggregator";

    public string UpstreamBaseUrl { get; set; } = "http://127.0.0.1:9000/";

    public int MaxConcurrency { get; set; } = 4;

    /// <summary>Timeout for one upstream attempt.</summary>
    public int CallTimeoutMs { get; set; } = 2000;

    /// <summary>Retries after the first attempt (so 2 means up to 3 calls).</summary>
    public int MaxRetries { get; set; } = 2;

    /// <summary>The delay before retry n (0-based) is <c>BackoffBaseMs * 2^n</c>.</summary>
    public int BackoffBaseMs { get; set; } = 100;

    /// <summary>Deadline for the whole <c>/aggregate</c> request.</summary>
    public int OverallTimeoutMs { get; set; } = 10_000;
}
