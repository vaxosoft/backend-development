using System.Threading.RateLimiting;
using Microsoft.Extensions.Options;

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
    public static IServiceCollection AddLinkRateLimiting(this IServiceCollection services) =>
        services.AddRateLimiter(limiter =>
        {
            limiter.OnRejected = async (context, cancellationToken) =>
            {
                var http = context.HttpContext;
                if (context.Lease.TryGetMetadata(MetadataName.RetryAfter, out var retryAfter))
                {
                    http.Response.Headers.RetryAfter = Math.Max(1, (int)Math.Ceiling(retryAfter.TotalSeconds)).ToString();
                }

                await Results.Problem(
                    statusCode: StatusCodes.Status429TooManyRequests,
                    title: "Too Many Requests",
                    detail: "link creation rate limit exceeded").ExecuteAsync(http);
            };

            limiter.AddPolicy(CreateLinkPolicy, http =>
            {
                var settings = http.RequestServices.GetRequiredService<IOptions<ShortenerOptions>>().Value;
                var userKey = http.User.GetUserId()?.ToString() ?? "anonymous";
                return RateLimitPartition.GetTokenBucketLimiter(userKey, _ => new TokenBucketRateLimiterOptions
                {
                    TokenLimit = settings.RateLimitBurst,
                    TokensPerPeriod = 1,
                    ReplenishmentPeriod = TimeSpan.FromSeconds(1 / settings.RateLimitRefillPerSecond),
                    QueueLimit = 0,
                    AutoReplenishment = true,
                });
            });
        });
}
