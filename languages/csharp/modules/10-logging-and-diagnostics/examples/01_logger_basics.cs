using Microsoft.Extensions.Logging;

namespace M10.Examples;

/// <summary>Example 1: levels, categories and why message templates are not string interpolation.</summary>
public static partial class LoggerBasics
{
    public static Task<int> RunAsync(string[] args)
    {
        var capture = new CaptureLoggerProvider();
        using var factory = LoggerFactory.Create(builder => builder
            .AddProvider(capture)
            .SetMinimumLevel(LogLevel.Information)
            .AddFilter("M10.Noisy", LogLevel.Warning)); // per-category override

        var orders = factory.CreateLogger("M10.Orders");
        var noisy = factory.CreateLogger("M10.Noisy");

        orders.LogDebug("below the minimum level, dropped");
        orders.LogInformation("Task {TaskId} assigned to {Owner}", 5, "ada");
        orders.LogWarning("Task {TaskId} is overdue by {Days} days", 5, 3);
        noisy.LogInformation("filtered out by the M10.Noisy category rule");
        noisy.LogWarning("kept: warnings pass the M10.Noisy rule");

        var taskId = 6;
        var owner = "grace";
        orders.LogInformation($"Task {taskId} assigned to {owner}"); // loses the structure: do not do this

        try
        {
            throw new InvalidOperationException("database is read-only");
        }
        catch (InvalidOperationException ex)
        {
            orders.LogError(ex, "Could not save task {TaskId}", 5);
        }

        LogCreated(orders, 7, "linus");

        foreach (var entry in capture.Entries)
        {
            var properties = string.Join(", ", entry.Properties.Select(p => $"{p.Key}={p.Value}"));
            Console.WriteLine($"{entry.Level,-11} {entry.Category,-10} {entry.Message}");
            Console.WriteLine($"            properties: {properties}");
            if (entry.Exception is not null)
            {
                Console.WriteLine($"            exception: {entry.Exception.GetType().Name}: {entry.Exception.Message}");
            }
        }

        Console.WriteLine($"orders.IsEnabled(Debug)={orders.IsEnabled(LogLevel.Debug)} noisy.IsEnabled(Information)={noisy.IsEnabled(LogLevel.Information)}");
        return Task.FromResult(0);
    }

    // Source-generated: no boxing, template parsed once, and the compiler checks the placeholders.
    [LoggerMessage(EventId = 1001, Level = LogLevel.Information, Message = "Task {TaskId} assigned to {Owner} (generated)")]
    private static partial void LogCreated(ILogger logger, int taskId, string owner);
}
