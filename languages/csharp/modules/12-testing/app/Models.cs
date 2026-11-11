using System.Text.Json;
using System.Text.Json.Serialization;

namespace M12.TaskApi;

public sealed record TaskItem(int Id, string Title, string Status, DateOnly? DueDate, DateTimeOffset CreatedAt);

public sealed record NewTask(string? Title, DateOnly? DueDate);

/// <summary>The seam between the HTTP layer and storage; tests replace it with fakes.</summary>
public interface ITaskStore
{
    Task<TaskItem> AddAsync(NewTask task, DateTimeOffset createdAt, CancellationToken ct = default);

    Task<TaskItem?> GetAsync(int id, CancellationToken ct = default);

    /// <summary>All tasks ordered by id, optionally only those with the given status.</summary>
    Task<IReadOnlyList<TaskItem>> ListAsync(string? status = null, CancellationToken ct = default);

    Task<TaskItem?> SetStatusAsync(int id, string status, CancellationToken ct = default);

    Task<bool> DeleteAsync(int id, CancellationToken ct = default);
}

/// <summary>Writes timestamps as RFC 3339 UTC (<c>2030-01-01T09:00:00Z</c>).</summary>
public sealed class UtcTimestampConverter : JsonConverter<DateTimeOffset>
{
    private const string Format = "yyyy-MM-dd'T'HH:mm:ss'Z'";

    public override DateTimeOffset Read(ref Utf8JsonReader reader, Type typeToConvert, JsonSerializerOptions options) =>
        DateTimeOffset.Parse(reader.GetString()!, null, System.Globalization.DateTimeStyles.AssumeUniversal);

    public override void Write(Utf8JsonWriter writer, DateTimeOffset value, JsonSerializerOptions options) =>
        writer.WriteStringValue(value.UtcDateTime.ToString(Format, System.Globalization.CultureInfo.InvariantCulture));
}
