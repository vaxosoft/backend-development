using System.Text.Json;
using Microsoft.AspNetCore.Http;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Logging;

namespace M10.Exercises;

public sealed record LogRecord(
    LogLevel Level,
    string Message,
    IReadOnlyDictionary<string, object?> Properties,
    Exception? Exception,
    IReadOnlyDictionary<string, object?> Scope)
{
    public string Template => (string)Properties["{OriginalFormat}"]!;
}

/// <summary>An in-memory ILogger that keeps the structured state and the active scopes of every entry.</summary>
public sealed class ListLogger<T> : ILogger<T>
{
    private readonly List<object> _scopes = [];

    public List<LogRecord> Records { get; } = [];

    public IDisposable? BeginScope<TState>(TState state) where TState : notnull
    {
        object boxed = state;
        _scopes.Add(boxed);
        return new Pop(this, boxed);
    }

    public bool IsEnabled(LogLevel logLevel) => true;

    public void Log<TState>(LogLevel logLevel, EventId eventId, TState state, Exception? exception, Func<TState, Exception?, string> formatter)
    {
        Records.Add(new LogRecord(logLevel, formatter(state, exception), Flatten(state), exception, FlattenScopes()));
    }

    private IReadOnlyDictionary<string, object?> FlattenScopes()
    {
        var all = new Dictionary<string, object?>();
        foreach (var scope in _scopes)
        {
            foreach (var pair in Flatten(scope))
            {
                all[pair.Key] = pair.Value;
            }
        }

        return all;
    }

    private static Dictionary<string, object?> Flatten(object? state) =>
        state is IEnumerable<KeyValuePair<string, object?>> pairs ? pairs.ToDictionary(p => p.Key, p => p.Value) : [];

    private sealed class Pop(ListLogger<T> owner, object state) : IDisposable
    {
        public void Dispose() => owner._scopes.RemoveAll(s => ReferenceEquals(s, state));
    }
}

public static class HttpTestExtensions
{
    public static DefaultHttpContext NewContext(string method = "GET", string path = "/tasks")
    {
        var context = new DefaultHttpContext { RequestServices = new ServiceCollection().BuildServiceProvider() };
        context.Request.Method = method;
        context.Request.Path = path;
        context.Response.Body = new MemoryStream();
        return context;
    }

    public static JsonDocument ReadJsonBody(this HttpContext context)
    {
        context.Response.Body.Position = 0;
        return JsonDocument.Parse(context.Response.Body);
    }

    public static string ReadBody(this HttpContext context)
    {
        context.Response.Body.Position = 0;
        return new StreamReader(context.Response.Body).ReadToEnd();
    }
}
