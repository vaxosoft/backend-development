namespace M13.Exercises;

/// <summary>Runs a periodic in-process job, the core of a scheduled background worker.</summary>
public static class Ex3_JobRunner
{
    /// <summary>
    /// Runs <paramref name="job"/> once per <paramref name="interval"/> (first run after one interval, never
    /// concurrently with itself) until <paramref name="ct"/> is cancelled. Timing comes from
    /// <paramref name="time"/>. A job that throws is reported to <paramref name="onError"/> and the schedule
    /// continues. Cancellation, even in the middle of a job, ends the returned task normally (no exception).
    /// </summary>
    public static async Task RunAsync(
        Func<CancellationToken, Task> job,
        TimeSpan interval,
        TimeProvider time,
        Action<Exception> onError,
        CancellationToken ct)
    {
        using var timer = new PeriodicTimer(interval, time);
        try
        {
            while (await timer.WaitForNextTickAsync(ct))
            {
                try
                {
                    await job(ct);
                }
                catch (Exception ex) when (ex is not OperationCanceledException)
                {
                    onError(ex);
                }
            }
        }
        catch (OperationCanceledException) when (ct.IsCancellationRequested)
        {
        }
    }
}
