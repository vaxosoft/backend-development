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
    public static Task RunAsync(
        Func<CancellationToken, Task> job,
        TimeSpan interval,
        TimeProvider time,
        Action<Exception> onError,
        CancellationToken ct) => throw new NotImplementedException();
}
