using System.Threading.RateLimiting;

namespace M11.Examples;

public static class RateLimiterDemo
{
    public static Task<int> RunAsync(string[] args)
    {
        // No background replenishment timer, so the output is deterministic; production leaves AutoReplenishment on.
        using var limiter = PartitionedRateLimiter.Create<string, string>(user =>
            RateLimitPartition.GetTokenBucketLimiter(user, _ => new TokenBucketRateLimiterOptions
            {
                TokenLimit = 3,
                TokensPerPeriod = 1,
                ReplenishmentPeriod = TimeSpan.FromSeconds(10),
                AutoReplenishment = false,
                QueueLimit = 0,
            }));

        void Try(string user)
        {
            using var lease = limiter.AttemptAcquire(user);
            var retry = lease.TryGetMetadata(MetadataName.RetryAfter, out var after)
                ? $" retry-after={after.TotalSeconds:0}s"
                : "";
            Console.WriteLine($"{user,-4} allowed={lease.IsAcquired}{retry}");
        }

        foreach (var _ in Enumerable.Range(0, 4))
        {
            Try("ada");
        }

        Try("bob");
        return Task.FromResult(0);
    }
}
