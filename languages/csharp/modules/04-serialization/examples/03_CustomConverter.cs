using System.Globalization;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace M04.Examples;

/// <summary>Custom JsonConverter&lt;T&gt;: RFC 3339 UTC timestamps with "Z" and a tolerant tag list.</summary>
public static class CustomConverter
{
    /// <summary>Writes <c>2030-01-01T09:00:00Z</c>; STJ's default writes <c>+00:00</c>.</summary>
    public sealed class UtcTimestampConverter : JsonConverter<DateTimeOffset>
    {
        private const string Format = "yyyy-MM-dd'T'HH:mm:ss'Z'";

        public override DateTimeOffset Read(ref Utf8JsonReader reader, Type typeToConvert, JsonSerializerOptions options)
        {
            var text = reader.GetString();
            if (text is null || !text.EndsWith('Z') ||
                !DateTimeOffset.TryParseExact(text, Format, CultureInfo.InvariantCulture, DateTimeStyles.AssumeUniversal, out var value))
            {
                throw new JsonException("expected an RFC 3339 UTC timestamp such as 2030-01-01T09:00:00Z");
            }

            return value;
        }

        public override void Write(Utf8JsonWriter writer, DateTimeOffset value, JsonSerializerOptions options) =>
            writer.WriteStringValue(value.UtcDateTime.ToString(Format, CultureInfo.InvariantCulture));
    }

    /// <summary>Accepts <c>["a","b"]</c> or <c>"a, b"</c>; always writes an array.</summary>
    public sealed class TagsConverter : JsonConverter<string[]>
    {
        public override string[] Read(ref Utf8JsonReader reader, Type typeToConvert, JsonSerializerOptions options)
        {
            switch (reader.TokenType)
            {
                case JsonTokenType.String:
                    return reader.GetString()!.Split(',', StringSplitOptions.TrimEntries | StringSplitOptions.RemoveEmptyEntries);
                case JsonTokenType.StartArray:
                    var tags = new List<string>();
                    while (reader.Read() && reader.TokenType != JsonTokenType.EndArray)
                    {
                        tags.Add(reader.GetString() ?? throw new JsonException("tags must not contain null"));
                    }

                    return [.. tags];
                default:
                    throw new JsonException("tags must be an array or a comma-separated string");
            }
        }

        public override void Write(Utf8JsonWriter writer, string[] value, JsonSerializerOptions options)
        {
            writer.WriteStartArray();
            foreach (var tag in value)
            {
                writer.WriteStringValue(tag);
            }

            writer.WriteEndArray();
        }
    }

    private static readonly JsonSerializerOptions Options = new(JsonSerializerDefaults.Web)
    {
        PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower,
        Converters =
        {
            new JsonStringEnumConverter(JsonNamingPolicy.SnakeCaseLower),
            new UtcTimestampConverter(),
            new TagsConverter(),
        },
    };

    public static Task<int> RunAsync(string[] args)
    {
        var created = new DateTimeOffset(2030, 1, 1, 11, 0, 0, TimeSpan.FromHours(2));
        var task = new TaskItem(1, "Write docs", Status.Todo, null, ["docs"], null, created);

        Console.WriteLine("default STJ : " + JsonSerializer.Serialize(task.CreatedAt));
        Console.WriteLine("converter   : " + JsonSerializer.Serialize(task.CreatedAt, Options));
        Console.WriteLine("task        : " + JsonSerializer.Serialize(task, Options));

        var fromString = JsonSerializer.Deserialize<TaskItem>(
            """{"id":2,"title":"t","status":"done","due_date":"2030-03-04","tags":"api, db ,","owner_id":null,"created_at":"2030-01-01T09:00:00Z"}""",
            Options)!;
        Console.WriteLine($"tags string : [{string.Join('|', fromString.Tags)}] due={fromString.DueDate:yyyy-MM-dd}");

        try
        {
            JsonSerializer.Deserialize<DateTimeOffset>("\"2030-01-01T09:00:00+02:00\"", Options);
        }
        catch (JsonException e)
        {
            Console.WriteLine("rejected    : " + e.Message.Split(" Path:")[0]);
        }

        return Task.FromResult(0);
    }
}
