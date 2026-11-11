using System.ComponentModel.DataAnnotations;

namespace P6.JobWorkerPool;

/// <summary>Pool settings, bound from the <c>Jobs</c> configuration section (env: <c>Jobs__Workers</c>, ...).</summary>
public sealed class JobsOptions
{
    public const string SectionName = "Jobs";

    [Range(1, 64)]
    public int Workers { get; set; } = 4;

    [Range(1, 100_000)]
    public int QueueCapacity { get; set; } = 100;

    [Range(1, 20)]
    public int MaxAttempts { get; set; } = 3;

    [Range(0, 60_000)]
    public int RetryDelayMs { get; set; } = 200;

    /// <summary>Timeout of a single attempt.</summary>
    [Range(1, 600_000)]
    public int JobTimeoutMs { get; set; } = 30_000;

    /// <summary>How long shutdown waits for running and queued jobs before cancelling them.</summary>
    [Range(0, 120_000)]
    public int DrainTimeoutMs { get; set; } = 10_000;
}
