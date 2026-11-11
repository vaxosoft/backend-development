#pragma warning disable CS9113 // starter: your implementation will read these parameters

namespace M11.Exercises;

public readonly record struct RateLimitDecision(bool Allowed, TimeSpan RetryAfter);

/// <summary>
/// One token bucket per key: starts full with <c>burst</c> tokens, refills continuously at
/// <c>refillPerSecond</c> and never holds more than <c>burst</c>. A request costs one token.
/// Safe to call from many threads.
/// </summary>
public sealed class Ex3_TokenBucketLimiter(int burst, double refillPerSecond, TimeProvider time)
{
    /// <summary>When denied, <c>RetryAfter</c> is how long until one whole token is available.</summary>
    public RateLimitDecision TryAcquire(string key) => throw new NotImplementedException();
}
