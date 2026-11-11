using System.Globalization;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace P2.TasksCrudApi;

/// <summary>A stored task, exactly as it is sent to clients (snake_case JSON).</summary>
public sealed record TaskItem(
    long Id,
    string Title,
    string Status,
    DateOnly? DueDate,
    IReadOnlyList<string> Tags,
    long? OwnerId,
    DateTimeOffset CreatedAt);

/// <summary>Body of POST and PUT. Everything is nullable so missing fields are validated, not rejected by the binder.</summary>
public sealed record TaskRequest(string? Title, string? Status, string? DueDate, List<string>? Tags, long? OwnerId);

/// <summary>A validated, normalised task ready to be stored.</summary>
public sealed record TaskData(string Title, string Status, DateOnly? DueDate, IReadOnlyList<string> Tags, long? OwnerId);

/// <summary>
/// Body of PATCH. Each <c>Has*</c> flag is true only when the property appeared in the JSON,
/// which is how "absent" (leave alone) is told apart from an explicit <c>null</c> (clear).
/// </summary>
public sealed class TaskPatch
{
    public string? Title { get; set { field = value; HasTitle = true; } }

    public string? Status { get; set { field = value; HasStatus = true; } }

    public string? DueDate { get; set { field = value; HasDueDate = true; } }

    public List<string>? Tags { get; set { field = value; HasTags = true; } }

    [JsonIgnore]
    public bool HasTitle { get; private set; }

    [JsonIgnore]
    public bool HasStatus { get; private set; }

    [JsonIgnore]
    public bool HasDueDate { get; private set; }

    [JsonIgnore]
    public bool HasTags { get; private set; }
}

/// <summary>Validated list parameters. <see cref="Sort"/> is one of the four whitelisted values.</summary>
public sealed record TaskQuery(string? Status, string? Tag, int Limit, int Offset, string Sort);

public sealed record TaskPage(IReadOnlyList<TaskItem> Items, int Total, int Limit, int Offset);

/// <summary>Writes timestamps as RFC 3339 UTC with second precision: <c>2030-01-01T09:00:00Z</c>.</summary>
public sealed class UtcTimestampConverter : JsonConverter<DateTimeOffset>
{
    private const string Format = "yyyy-MM-dd'T'HH:mm:ss'Z'";

    public override DateTimeOffset Read(ref Utf8JsonReader reader, Type typeToConvert, JsonSerializerOptions options) =>
        DateTimeOffset.Parse(reader.GetString()!, CultureInfo.InvariantCulture, DateTimeStyles.AssumeUniversal);

    public override void Write(Utf8JsonWriter writer, DateTimeOffset value, JsonSerializerOptions options) =>
        writer.WriteStringValue(value.UtcDateTime.ToString(Format, CultureInfo.InvariantCulture));
}
