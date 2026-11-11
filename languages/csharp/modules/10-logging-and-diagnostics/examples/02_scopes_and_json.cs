using Microsoft.Extensions.Logging;

namespace M10.Examples;

/// <summary>Example 2: scopes add context to every line written inside them; the JSON formatter keeps it machine-readable.</summary>
public static class ScopesAndJson
{
    public static Task<int> RunAsync(string[] args)
    {
        Console.WriteLine("JSON lines written by the console logger:");

        using (var factory = LoggerFactory.Create(builder => builder
            .SetMinimumLevel(LogLevel.Information)
            .AddJsonConsole(options =>
            {
                options.IncludeScopes = true;
                options.JsonWriterOptions = new() { Indented = false };
            })))
        {
            var logger = factory.CreateLogger("TaskService");

            logger.LogInformation("Service started");

            using (logger.BeginScope("request {RequestId} by user {UserId}", "req-42", 7))
            {
                logger.LogInformation("Task {TaskId} created", 5);

                using (logger.BeginScope("importing {Count} tasks", 3))
                {
                    logger.LogWarning("Task {TaskId} skipped: {Reason}", 6, "duplicate title");
                }
            }

            logger.LogInformation("Back outside every scope");
        } // disposing the factory flushes the console logger's background queue

        return Task.FromResult(0);
    }
}
