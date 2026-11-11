using System.Text.Json;
using System.Text.Json.Serialization;

namespace M08.Examples;

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

public static class Json
{
    public static readonly JsonSerializerOptions Options = new(JsonSerializerDefaults.Web)
    {
        PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower,
        Encoder = System.Text.Encodings.Web.JavaScriptEncoder.UnsafeRelaxedJsonEscaping,
    };
}
