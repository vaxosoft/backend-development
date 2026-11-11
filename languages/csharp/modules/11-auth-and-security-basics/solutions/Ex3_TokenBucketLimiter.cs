using System.Collections.Concurrent;

namespace M11.Exercises;

public readonly record struct RateLimitDecision(bool Allowed, TimeSpan RetryAfter);

/// <summary>
/// One token bucket per key: starts full with <c>burst</c> tokens, refills continuously at
/// <c>refillPerSecond</c> and never holds more than <c>burst</c>. A request costs one token.
/// Safe to call from many threads.
/// </summary>
public sealed class Ex3_TokenBucketLimiter
{
    private sealed class Bucket(double tokens, long timestamp)
    {
        public double Tokens = tokens;
        public long Timestamp = timestamp;
    }

    private readonly int _burst;
    private readonly double _refillPerSecond;
    private readonly TimeProvider _time;
    private readonly ConcurrentDictionary<string, Bucket> _buckets = new();

    public Ex3_TokenBucketLimiter(int burst, double refillPerSecond, TimeProvider time)
    {
        ArgumentOutOfRangeException.ThrowIfLessThan(burst, 1);
        ArgumentOutOfRangeException.ThrowIfLessThanOrEqual(refillPerSecond, 0);
        _burst = burst;
        _refillPerSecond = refillPerSecond;
        _time = time;
    }

    /// <summary>When denied, <c>RetryAfter</c> is how long until one whole token is available.</summary>
    public RateLimitDecision TryAcquire(string key)
    {
        var now = _time.GetTimestamp();
        var bucket = _buckets.GetOrAdd(key, _ => new Bucket(_burst, now));
        lock (bucket)
        {
            var elapsed = _time.GetElapsedTime(bucket.Timestamp, now).TotalSeconds;
            bucket.Tokens = Math.Min(_burst, bucket.Tokens + elapsed * _refillPerSecond);
            bucket.Timestamp = now;

            if (bucket.Tokens >= 1)
            {
                bucket.Tokens -= 1;
                return new RateLimitDecision(true, TimeSpan.Zero);
            }

            return new RateLimitDecision(false, TimeSpan.FromSeconds((1 - bucket.Tokens) / _refillPerSecond));
        }
    }
}
