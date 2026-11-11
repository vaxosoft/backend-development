using System.Text.Json;
using System.Text.Json.Serialization;

namespace M07.Examples;

[JsonConverter(typeof(JsonStringEnumConverter<TaskState>))]
public enum TaskState
{
    [JsonStringEnumMemberName("todo")] Todo,
    [JsonStringEnumMemberName("in_progress")] InProgress,
    [JsonStringEnumMemberName("done")] Done,
}

public sealed record TaskItem(
    int Id,
    string Title,
    TaskState Status,
    DateOnly? DueDate,
    string[] Tags,
    int? OwnerId,
    DateTimeOffset CreatedAt);

public sealed record NewTask(string Title, DateOnly? DueDate = null, string[]? Tags = null);

public static class Json
{
    /// <summary>Wire format shared by every module: snake_case members, ISO 8601 dates.</summary>
    public static readonly JsonSerializerOptions Options = Configure(new JsonSerializerOptions(JsonSerializerDefaults.Web));

    public static JsonSerializerOptions Configure(JsonSerializerOptions options)
    {
        options.PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower;
        return options;
    }
}
