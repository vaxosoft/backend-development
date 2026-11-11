using System.Text.Json;
using System.Text.Json.Serialization;

namespace M04.Examples;

/// <summary>Task Tracker wire contract: source-generated serializer, strict decoding, validation to 400/422.</summary>
public static partial class TaskTrackerJson
{
    [JsonConverter(typeof(JsonStringEnumConverter<State>))]
    public enum State
    {
        [JsonStringEnumMemberName("todo")] Todo,
        [JsonStringEnumMemberName("in_progress")] InProgress,
        [JsonStringEnumMemberName("done")] Done,
    }

    public sealed record NewTask(string? Title, State? Status, DateOnly? DueDate, string[]? Tags, int? OwnerId);

    public sealed record TaskView(int Id, string Title, State Status, DateOnly? DueDate, string[] Tags, int? OwnerId, DateTimeOffset CreatedAt);

    public sealed record Problem(int Status, string Code, string Message, Dictionary<string, string>? Fields);

    [JsonSourceGenerationOptions(
        JsonSerializerDefaults.Web,
        PropertyNamingPolicy = JsonKnownNamingPolicy.SnakeCaseLower,
        UnmappedMemberHandling = JsonUnmappedMemberHandling.Disallow,
        DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull,
        Converters = [typeof(CustomConverter.UtcTimestampConverter)])]
    [JsonSerializable(typeof(NewTask))]
    [JsonSerializable(typeof(TaskView))]
    [JsonSerializable(typeof(Problem))]
    internal sealed partial class WireContext : JsonSerializerContext;

    private static readonly DateTimeOffset FixedNow = new(2030, 1, 1, 9, 0, 0, TimeSpan.Zero);

    public static Task<int> RunAsync(string[] args)
    {
        string[] bodies =
        [
            """{"title":"  Write docs  ","due_date":"2030-01-31","tags":["docs","p1"]}""",
            """{"title":"No tags, defaults apply"}""",
            """{"title":"x",""",
            """{"titel":"typo"}""",
            """{"title":"x","status":"finished"}""",
            """{"title":"   ","tags":["Docs","docs","docs"]}""",
        ];

        var nextId = 1;
        foreach (var body in bodies)
        {
            Console.WriteLine($"> {body}");
            Console.WriteLine("< " + Handle(body, ref nextId));
        }

        return Task.FromResult(0);
    }

    private static string Handle(string body, ref int nextId)
    {
        NewTask? request;
        try
        {
            request = JsonSerializer.Deserialize(body, WireContext.Default.NewTask);
        }
        catch (JsonException e)
        {
            return JsonSerializer.Serialize(
                new Problem(400, "bad_request", $"invalid JSON at {e.Path}", null), WireContext.Default.Problem);
        }

        var fields = Validate(request!);
        if (fields.Count > 0)
        {
            return JsonSerializer.Serialize(
                new Problem(422, "validation_failed", "invalid task", fields), WireContext.Default.Problem);
        }

        var view = new TaskView(
            nextId++, request!.Title!.Trim(), request.Status ?? State.Todo, request.DueDate,
            request.Tags ?? [], request.OwnerId, FixedNow);
        return "201 " + JsonSerializer.Serialize(view, WireContext.Default.TaskView);
    }

    private static Dictionary<string, string> Validate(NewTask task)
    {
        var fields = new Dictionary<string, string>();
        if (task.Title?.Trim() is not { Length: >= 1 and <= 200 })
        {
            fields["title"] = "must be 1-200 characters";
        }

        if (task.Tags is { } tags)
        {
            if (tags.Length > 10)
            {
                fields["tags"] = "at most 10 tags";
            }
            else if (tags.Any(t => t.Length is < 1 or > 30 || t != t.ToLowerInvariant()))
            {
                fields["tags"] = "each tag must be 1-30 lowercase characters";
            }
            else if (tags.Distinct().Count() != tags.Length)
            {
                fields["tags"] = "tags must be unique";
            }
        }

        return fields;
    }
}
