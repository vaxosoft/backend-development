using System.Text.Json;

namespace M04.Exercises;

public static class Ex1_Wire
{
    /// <summary>
    /// Options matching the Task Tracker wire format: snake_case members, snake_case string enums
    /// ("in_progress"), case-insensitive reading, nulls written explicitly, unknown members ignored.
    /// </summary>
    public static JsonSerializerOptions CreateOptions() => throw new NotImplementedException();

    public static string ToJson(TaskDto task) => JsonSerializer.Serialize(task, CreateOptions());

    public static TaskDto FromJson(string json) =>
        JsonSerializer.Deserialize<TaskDto>(json, CreateOptions()) ?? throw new JsonException("body must not be null");
}
