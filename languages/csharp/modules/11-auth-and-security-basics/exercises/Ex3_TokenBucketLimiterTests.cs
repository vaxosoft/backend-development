using Microsoft.Extensions.Time.Testing;

namespace M11.Exercises;

public class Ex3_TokenBucketLimiterTests
{
    private readonly FakeTimeProvider _clock = new();

    [Fact]
    public void AllowsABurstThenDenies()
    {
        var limiter = new Ex3_TokenBucketLimiter(3, 1, _clock);

        Assert.All(Enumerable.Range(0, 3), _ => Assert.True(limiter.TryAcquire("ada").Allowed));

        var denied = limiter.TryAcquire("ada");
        Assert.False(denied.Allowed);
        Assert.Equal(TimeSpan.FromSeconds(1), denied.RetryAfter);
    }

    [Fact]
    public void RefillsContinuouslyAndReportsTheRemainingWait()
    {
        var limiter = new Ex3_TokenBucketLimiter(1, 1, _clock);
        Assert.True(limiter.TryAcquire("ada").Allowed);

        _clock.Advance(TimeSpan.FromMilliseconds(250));
        Assert.Equal(TimeSpan.FromMilliseconds(750), limiter.TryAcquire("ada").RetryAfter);

        _clock.Advance(TimeSpan.FromMilliseconds(750));
        Assert.True(limiter.TryAcquire("ada").Allowed);
    }

    [Fact]
    public void SlowRefillRatesAreSupported()
    {
        var limiter = new Ex3_TokenBucketLimiter(1, 0.5, _clock);
        limiter.TryAcquire("ada");

        Assert.Equal(TimeSpan.FromSeconds(2), limiter.TryAcquire("ada").RetryAfter);
    }

    [Fact]
    public void NeverAccumulatesMoreThanTheBurst()
    {
        var limiter = new Ex3_TokenBucketLimiter(2, 1, _clock);
        _clock.Advance(TimeSpan.FromHours(1));

        Assert.True(limiter.TryAcquire("ada").Allowed);
        Assert.True(limiter.TryAcquire("ada").Allowed);
        Assert.False(limiter.TryAcquire("ada").Allowed);
    }

    [Fact]
    public void KeysAreIndependent()
    {
        var limiter = new Ex3_TokenBucketLimiter(1, 1, _clock);

        Assert.True(limiter.TryAcquire("ada").Allowed);
        Assert.False(limiter.TryAcquire("ada").Allowed);
        Assert.True(limiter.TryAcquire("bob").Allowed);
    }

    [Fact]
    public void IsExactUnderConcurrency()
    {
        var limiter = new Ex3_TokenBucketLimiter(10, 1, _clock);
        var allowed = 0;

        Parallel.For(0, 200, _ =>
        {
            if (limiter.TryAcquire("shared").Allowed)
            {
                Interlocked.Increment(ref allowed);
            }
        });

        Assert.Equal(10, allowed);
    }

    [Theory]
    [InlineData(0, 1)]
    [InlineData(1, 0)]
    [InlineData(1, -1)]
    public void RejectsNonsensicalSettings(int burst, double rate) =>
        Assert.Throws<ArgumentOutOfRangeException>(() => new Ex3_TokenBucketLimiter(burst, rate, _clock));
}
