using System.Text.Json;
using System.Text.Json.Serialization;

namespace M04.Examples;

/// <summary>Serialize and deserialize with System.Text.Json; naming policies and defaults.</summary>
public static class JsonBasics
{
    private static readonly JsonSerializerOptions Wire = new(JsonSerializerDefaults.Web)
    {
        PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower,
        Converters = { new JsonStringEnumConverter(JsonNamingPolicy.SnakeCaseLower) },
    };

    public static Task<int> RunAsync(string[] args)
    {
        var task = new TaskItem(
            1, "Write docs", Status.InProgress, new DateOnly(2030, 1, 31),
            ["docs", "p1"], null, new DateTimeOffset(2030, 1, 1, 9, 0, 0, TimeSpan.Zero));

        Console.WriteLine("defaults : " + JsonSerializer.Serialize(task));
        Console.WriteLine("wire     : " + JsonSerializer.Serialize(task, Wire));

        var back = JsonSerializer.Deserialize<TaskItem>(
            """{"ID":2,"Title":"Case does not matter","status":"done","due_date":null,"tags":[],"owner_id":7,"created_at":"2030-02-01T00:00:00Z"}""",
            Wire);
        Console.WriteLine($"decoded  : id={back!.Id} status={back.Status} due={back.DueDate?.ToString("O") ?? "null"} owner={back.OwnerId} created={back.CreatedAt:O}");

        Console.WriteLine("anonymous: " + JsonSerializer.Serialize(new { ok = true, n = 3 }, Wire));
        return Task.FromResult(0);
    }
}
