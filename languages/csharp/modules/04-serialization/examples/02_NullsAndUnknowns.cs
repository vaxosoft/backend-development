using System.Text.Json;
using System.Text.Json.Serialization;

namespace M04.Examples;

/// <summary>Attributes, required members, null handling, unknown fields and JsonDocument.</summary>
public static class NullsAndUnknowns
{
    public sealed class CreateTask
    {
        public required string Title { get; init; }

        [JsonPropertyName("due_date")]
        public DateOnly? DueDate { get; init; }

        public List<string> Tags { get; init; } = [];

        [JsonIgnore]
        public string Internal { get; init; } = "never on the wire";
    }

    public sealed class Summary
    {
        public string Title { get; init; } = "";

        [JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)]
        public string? Note { get; init; }
    }

    private static readonly JsonSerializerOptions Lenient = new(JsonSerializerDefaults.Web)
    {
        PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower,
    };

    private static readonly JsonSerializerOptions Strict = new(Lenient)
    {
        UnmappedMemberHandling = JsonUnmappedMemberHandling.Disallow,
    };

    public static Task<int> RunAsync(string[] args)
    {
        var ok = JsonSerializer.Deserialize<CreateTask>(
            """{"title":"Ship","extra":"ignored","due_date":null}""", Lenient)!;
        Console.WriteLine($"unknown field ignored: {ok.Title}, due={(ok.DueDate is null ? "null" : "set")}, tags={ok.Tags.Count}, {ok.Internal}");

        Try("missing required   ", """{"tags":["a"]}""", Lenient);
        Try("unknown disallowed ", """{"title":"x","extra":1}""", Strict);
        Try("bad date           ", """{"title":"x","due_date":"31/01/2030"}""", Lenient);
        Try("wrong type         ", """{"title":42}""", Lenient);
        Try("malformed          ", """{"title":"x",""", Lenient);

        Console.WriteLine(JsonSerializer.Serialize(new Summary { Title = "t" }, Lenient));
        Console.WriteLine(JsonSerializer.Serialize(new Summary { Title = "t", Note = "n" }, Lenient));

        // JsonDocument: read without a type; null, absent and value are distinguishable.
        using var doc = JsonDocument.Parse("""{"due_date":null,"title":"x"}""");
        foreach (var name in new[] { "title", "due_date", "owner_id" })
        {
            var state = !doc.RootElement.TryGetProperty(name, out var el) ? "absent"
                : el.ValueKind == JsonValueKind.Null ? "null" : $"value {el}";
            Console.WriteLine($"{name,-9}: {state}");
        }

        return Task.FromResult(0);
    }

    private static void Try(string label, string json, JsonSerializerOptions options)
    {
        try
        {
            var t = JsonSerializer.Deserialize<CreateTask>(json, options);
            Console.WriteLine($"{label}: ok {t?.Title}");
        }
        catch (JsonException e)
        {
            Console.WriteLine($"{label}: {e.GetType().Name}: {e.Message.Split(" Path:")[0]}");
        }
    }
}
