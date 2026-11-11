using Microsoft.Extensions.Time.Testing;

namespace M13.Exercises;

public class Ex3_JobRunnerTests
{
    private static readonly TimeSpan Interval = TimeSpan.FromMinutes(5);
    private static readonly TimeSpan Patience = TimeSpan.FromSeconds(5);

    private readonly FakeTimeProvider _clock = new();
    private readonly CancellationTokenSource _cts = new();
    private readonly SemaphoreSlim _ran = new(0);
    private readonly List<Exception> _errors = [];

    private Task Start(Func<CancellationToken, Task> job) =>
        Ex3_JobRunner.RunAsync(job, Interval, _clock, _errors.Add, _cts.Token);

    private async Task WaitForRun() => Assert.True(await _ran.WaitAsync(Patience), "job did not run in time");

    [Fact]
    public async Task RunsOncePerInterval_NotAtStartup()
    {
        var runs = 0;
        var runner = Start(_ =>
        {
            Interlocked.Increment(ref runs);
            _ran.Release();
            return Task.CompletedTask;
        });

        _clock.Advance(Interval - TimeSpan.FromSeconds(1));
        Assert.Equal(0, Volatile.Read(ref runs));

        _clock.Advance(TimeSpan.FromSeconds(1));
        await WaitForRun();
        _clock.Advance(Interval);
        await WaitForRun();

        Assert.Equal(2, runs);
        await _cts.CancelAsync();
        await runner;
    }

    [Fact]
    public async Task AFailingJobIsReportedAndTheScheduleContinues()
    {
        var calls = 0;
        var runner = Start(_ =>
        {
            var call = Interlocked.Increment(ref calls);
            _ran.Release();
            return call == 1 ? Task.FromException(new InvalidOperationException("boom")) : Task.CompletedTask;
        });

        _clock.Advance(Interval);
        await WaitForRun();
        _clock.Advance(Interval);
        await WaitForRun();
        await _cts.CancelAsync();
        await runner;

        Assert.Equal("boom", Assert.Single(_errors).Message);
        Assert.Equal(2, calls);
    }

    [Fact]
    public async Task CancellingWhileIdleCompletesNormally()
    {
        var runner = Start(_ => Task.CompletedTask);

        await _cts.CancelAsync();
        await runner.WaitAsync(Patience);

        Assert.True(runner.IsCompletedSuccessfully);
        Assert.Empty(_errors);
    }

    [Fact]
    public async Task CancellingInsideARunningJobCompletesNormally()
    {
        var runner = Start(async ct =>
        {
            _ran.Release();
            await Task.Delay(Timeout.InfiniteTimeSpan, ct);
        });

        _clock.Advance(Interval);
        await WaitForRun();
        await _cts.CancelAsync();
        await runner.WaitAsync(Patience);

        Assert.True(runner.IsCompletedSuccessfully);
        Assert.Empty(_errors);
    }

    [Fact]
    public async Task NeverRunsTheJobConcurrentlyWithItself()
    {
        var release = new SemaphoreSlim(0);
        int running = 0, maxRunning = 0, started = 0;
        var runner = Start(async ct =>
        {
            var now = Interlocked.Increment(ref running);
            InterlockedMax(ref maxRunning, now);
            Interlocked.Increment(ref started);
            _ran.Release();
            await release.WaitAsync(ct);
            Interlocked.Decrement(ref running);
        });

        _clock.Advance(Interval);
        await WaitForRun();
        _clock.Advance(Interval);
        _clock.Advance(Interval);
        Assert.Equal(1, Volatile.Read(ref started));

        release.Release();
        await WaitForRun();
        release.Release();
        await _cts.CancelAsync();
        await runner.WaitAsync(Patience);

        Assert.Equal(1, maxRunning);
    }

    private static void InterlockedMax(ref int target, int value)
    {
        int current;
        while ((current = Volatile.Read(ref target)) < value && Interlocked.CompareExchange(ref target, value, current) != current)
        {
        }
    }
}
