using Microsoft.Extensions.Options;

namespace P6.JobWorkerPool;

/// <summary>A fixed number of workers pulling jobs from <see cref="JobService"/>, with retries, timeouts and a bounded drain on shutdown.</summary>
public sealed class WorkerPool(
    JobService jobs,
    IOptions<JobsOptions> options,
    TimeProvider time,
    ILogger<WorkerPool> logger) : BackgroundService
{
    private readonly JobsOptions _options = options.Value;

    /// <param name="abort">Cancelled only after the drain timeout: it means "stop now", not "begin shutdown".</param>
    protected override Task ExecuteAsync(CancellationToken abort) =>
        Task.WhenAll(Enumerable.Range(1, _options.Workers).Select(_ => Task.Run(() => WorkerLoopAsync(abort), CancellationToken.None)));

    public override async Task StopAsync(CancellationToken cancellationToken)
    {
        jobs.StopAccepting();

        // TODO (drain): wait for the workers to finish (they drain the queue), but at most DrainTimeoutMs.
        // Task.WaitAsync(TimeSpan, TimeProvider, CancellationToken) throws TimeoutException when the time is up.
        // Only then call base.StopAsync, which cancels the token passed to ExecuteAsync.
        await base.StopAsync(cancellationToken);
    }

    private async Task WorkerLoopAsync(CancellationToken abort)
    {
        // The loop ends when the queue is completed and empty, which is exactly "drained".
        await foreach (var job in jobs.Reader.ReadAllAsync(CancellationToken.None))
        {
            if (abort.IsCancellationRequested)
            {
                job.TryCancel();
                continue;
            }

            await RunAsync(job, abort);
        }
    }

    private async Task RunAsync(Job job, CancellationToken abort)
    {
        // TODO (RunAsync): run the job. Rules:
        //  - job.TryStart() is false when the job was cancelled while queued: do nothing.
        //  - up to _options.MaxAttempts attempts, each calling job.BeginAttempt() and JobHandlers.ExecuteAsync(...).
        //  - each attempt gets a timeout of JobTimeoutMs (use `time`); a timeout is a failed attempt, not a cancellation.
        //  - job.Token (DELETE) or abort (shutdown) cancelling the job ends it: job.TryCancel() and return.
        //  - between attempts wait RetryDelayMs (Task.Delay with `time`); after the last failure call job.Fail(error).
        //  - success: job.Succeed(result). Unexpected exceptions must not escape (they would stop the host).
        _ = (job, abort, time, logger);
        await Task.CompletedTask;
    }
}
