namespace P1.HelloHttp;

/// <summary>One finished request, as seen by the logging middleware.</summary>
public sealed record RequestLogEntry(string Method, string Path, int Status, TimeSpan Duration, string RequestId);

/// <summary>Where request log lines go. Tests replace it to capture entries.</summary>
public interface IRequestLogSink
{
    void Write(RequestLogEntry entry);
}

/// <summary>Default sink: one structured line per request through ILogger.</summary>
public sealed class LoggerRequestLogSink(ILogger<LoggerRequestLogSink> logger) : IRequestLogSink
{
    public void Write(RequestLogEntry entry) =>
        logger.LogInformation(
            "{Method} {Path} -> {Status} in {DurationMs:0.0}ms [{RequestId}]",
            entry.Method, entry.Path, entry.Status, entry.Duration.TotalMilliseconds, entry.RequestId);
}
