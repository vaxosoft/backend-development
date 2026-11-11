using Microsoft.Extensions.Logging;

namespace M10.Examples;

public sealed record LogEntry(LogLevel Level, string Category, string Message, Dictionary<string, object?> Properties, Exception? Exception);

/// <summary>Keeps log entries in memory so examples can show their structure deterministically.</summary>
public sealed class CaptureLoggerProvider : ILoggerProvider
{
    private readonly List<LogEntry> _entries = [];

    public IReadOnlyList<LogEntry> Entries
    {
        get
        {
            lock (_entries)
            {
                return [.. _entries];
            }
        }
    }

    public ILogger CreateLogger(string categoryName) => new CaptureLogger(categoryName, this);

    public void Dispose()
    {
    }

    private sealed class CaptureLogger(string category, CaptureLoggerProvider owner) : ILogger
    {
        public IDisposable? BeginScope<TState>(TState state) where TState : notnull => null;

        public bool IsEnabled(LogLevel logLevel) => logLevel != LogLevel.None;

        public void Log<TState>(LogLevel logLevel, EventId eventId, TState state, Exception? exception, Func<TState, Exception?, string> formatter)
        {
            var properties = state is IEnumerable<KeyValuePair<string, object?>> pairs
                ? pairs.ToDictionary(p => p.Key, p => p.Value)
                : [];
            lock (owner._entries)
            {
                owner._entries.Add(new LogEntry(logLevel, category, formatter(state, exception), properties, exception));
            }
        }
    }
}
