using System.Text.Json;
using System.Text.Json.Serialization;

namespace M14.Examples;

public sealed record TaskDto(int Id, string Title, string Status, string? DueDate);

public sealed record NotRegistered(string Name);

/// <summary>Compile-time serialization metadata: no runtime reflection, safe for trimming and Native AOT.</summary>
[JsonSourceGenerationOptions(PropertyNamingPolicy = JsonKnownNamingPolicy.SnakeCaseLower)]
[JsonSerializable(typeof(TaskDto))]
[JsonSerializable(typeof(TaskDto[]))]
[JsonSerializable(typeof(HealthDto))]
[JsonSerializable(typeof(VersionDto))]
public sealed partial class AppJson : JsonSerializerContext;

public sealed record HealthDto(string Status);

public sealed record VersionDto(string Name, string Version);

public static class JsonSourceGen
{
    public static Task<int> RunAsync(string[] args)
    {
        var task = new TaskDto(1, "Write docs", "todo", "2030-01-31");
        Console.WriteLine(JsonSerializer.Serialize(task, AppJson.Default.TaskDto));

        // Reflection-based serialization is what trimming removes: with only the generated context
        // as resolver, a type that is not registered cannot be serialized at all.
        var strict = new JsonSerializerOptions { TypeInfoResolver = AppJson.Default };
        Console.WriteLine($"TaskDto registered: {strict.TryGetTypeInfo(typeof(TaskDto), out _)}");
        Console.WriteLine($"NotRegistered registered: {strict.TryGetTypeInfo(typeof(NotRegistered), out _)} (add [JsonSerializable])");

        return Task.FromResult(0);
    }
}
