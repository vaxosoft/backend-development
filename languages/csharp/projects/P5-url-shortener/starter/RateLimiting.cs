using System.Threading.RateLimiting;

namespace P5.UrlShortener;

public static class RateLimiting
{
    public const string CreateLinkPolicy = "create-link";

    /// <summary>
    /// Registers the <see cref="CreateLinkPolicy"/> policy: a token bucket per authenticated user
    /// (capacity <see cref="ShortenerOptions.RateLimitBurst"/>, refilled by
    /// <see cref="ShortenerOptions.RateLimitRefillPerSecond"/> tokens per second, no queueing).
    /// Rejected requests get 429 <c>application/problem+json</c> and a <c>Retry-After</c> header (whole seconds, at least 1).
    /// </summary>
    public static IServiceCollection AddLinkRateLimiting(this IServiceCollection services)
    {
        // TODO(P5): AddRateLimiter + AddPolicy(CreateLinkPolicy, ...) with RateLimitPartition.GetTokenBucketLimiter,
        // partitioned by user id; set RejectionStatusCode/OnRejected. This placeholder lets everything through.
        return services.AddRateLimiter(limiter =>
            limiter.AddPolicy(CreateLinkPolicy, _ => RateLimitPartition.GetNoLimiter("todo")));
    }
}
