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

        if (ExecuteTask is { } workers)
        {
            try
            {
                await workers.WaitAsync(TimeSpan.FromMilliseconds(_options.DrainTimeoutMs), time, cancellationToken);
            }
            catch (Exception ex) when (ex is TimeoutException or OperationCanceledException)
            {
                logger.LogWarning("Drain timeout of {DrainTimeoutMs} ms elapsed, cancelling the remaining jobs", _options.DrainTimeoutMs);
            }
        }

        // Cancels the token handed to ExecuteAsync and waits for the workers to return.
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
        if (!job.TryStart())
        {
            return;
        }

        using var stop = CancellationTokenSource.CreateLinkedTokenSource(job.Token, abort);
        var error = "unknown error";

        for (var attempt = 1; attempt <= _options.MaxAttempts; attempt++)
        {
            job.BeginAttempt();
            using var timeout = new CancellationTokenSource(TimeSpan.FromMilliseconds(_options.JobTimeoutMs), time);
            using var attemptCts = CancellationTokenSource.CreateLinkedTokenSource(stop.Token, timeout.Token);

            try
            {
                job.Succeed(await JobHandlers.ExecuteAsync(job, time, attemptCts.Token));
                return;
            }
            catch (OperationCanceledException) when (stop.IsCancellationRequested)
            {
                job.TryCancel();
                return;
            }
            catch (OperationCanceledException)
            {
                error = $"timed out after {_options.JobTimeoutMs} ms";
            }
            catch (JobFailedException ex)
            {
                error = ex.Message;
            }
            catch (Exception ex)
            {
                logger.LogError(ex, "Job {JobId} crashed", job.Id);
                error = "internal error";
            }

            logger.LogWarning("Job {JobId} attempt {Attempt}/{MaxAttempts} failed: {Error}", job.Id, attempt, _options.MaxAttempts, error);

            if (attempt < _options.MaxAttempts)
            {
                try
                {
                    await Task.Delay(TimeSpan.FromMilliseconds(_options.RetryDelayMs), time, stop.Token);
                }
                catch (OperationCanceledException)
                {
                    job.TryCancel();
                    return;
                }
            }
        }

        job.Fail(error);
        logger.LogError("Job {JobId} failed after {Attempts} attempts: {Error}", job.Id, _options.MaxAttempts, error);
    }
}
