using System.ComponentModel.DataAnnotations;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace M03.Examples;

public enum TaskState
{
    Todo,
    InProgress,
    Done,
}

public static class TaskStates
{
    /// <summary>Strict wire names; Enum.TryParse would also accept "1" or "InProgress".</summary>
    public static bool TryParse(string? wire, out TaskState state)
    {
        (var ok, state) = wire switch
        {
            "todo" => (true, TaskState.Todo),
            "in_progress" => (true, TaskState.InProgress),
            "done" => (true, TaskState.Done),
            _ => (false, default(TaskState)),
        };
        return ok;
    }
}

/// <summary>What the API returns. Never the storage type, never the request type.</summary>
public sealed record TaskItem(
    int Id,
    string Title,
    TaskState Status,
    DateOnly? DueDate,
    IReadOnlyList<string> Tags,
    int? OwnerId,
    DateTime CreatedAt);

/// <summary>What clients may send. No Id and no CreatedAt: they cannot be over-posted.</summary>
public sealed record TaskInput
{
    [Required(AllowEmptyStrings = true, ErrorMessage = "is required"), TrimmedLength(1, 200)]
    public string? Title { get; init; }

    public TaskState Status { get; init; } = TaskState.Todo;

    public DateOnly? DueDate { get; init; }

    [TagList]
    public string[] Tags { get; init; } = [];

    public int? OwnerId { get; init; }
}

[AttributeUsage(AttributeTargets.Property)]
public sealed class TrimmedLengthAttribute(int min, int max) : ValidationAttribute
{
    public override bool IsValid(object? value) =>
        value is string s && s.Trim().Length >= min && s.Trim().Length <= max;

    public override string FormatErrorMessage(string name) => $"must be {min}-{max} characters";
}

[AttributeUsage(AttributeTargets.Property)]
public sealed class TagListAttribute : ValidationAttribute
{
    protected override ValidationResult? IsValid(object? value, ValidationContext context)
    {
        if (value is not string[] tags)
        {
            return ValidationResult.Success;
        }

        var error = tags switch
        {
            { Length: > 10 } => "at most 10 tags",
            _ when tags.Any(t => t.Length is < 1 or > 30) => "each tag must be 1-30 characters",
            _ when tags.Any(t => t != t.ToLowerInvariant()) => "every tag must be lowercase",
            _ when tags.Distinct().Count() != tags.Length => "tags must be unique",
            _ => null,
        };
        return error is null ? ValidationResult.Success : new ValidationResult(error, [context.MemberName!]);
    }
}

static class Json
{
    /// <summary>snake_case names and string enums: the Task Tracker wire format.</summary>
    public static void Configure(JsonSerializerOptions options)
    {
        options.PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower;
        options.Converters.Add(new JsonStringEnumConverter(JsonNamingPolicy.SnakeCaseLower));
    }
}
