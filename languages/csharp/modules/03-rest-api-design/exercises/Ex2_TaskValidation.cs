using System.Globalization;
using System.Text.Json.Serialization;

namespace M03.Exercises;

/// <summary>Body of POST /tasks. Everything is nullable so that "missing" is a validation error, not a crash.</summary>
public sealed record CreateTaskRequest(
    string? Title,
    string? Status,
    [property: JsonPropertyName("due_date")] string? DueDate,
    string[]? Tags,
    [property: JsonPropertyName("owner_id")] int? OwnerId);

public static class TaskValidator
{
    /// <summary>
    /// Returns field name (snake_case, as on the wire) -> messages. An empty dictionary means "valid".
    /// <list type="bullet">
    /// <item>title: missing -> "is required"; blank after trimming or longer than 200 trimmed characters -> "must be 1-200 characters"</item>
    /// <item>status (optional): not todo/in_progress/done -> "must be one of: todo, in_progress, done"</item>
    /// <item>due_date (optional): not a real yyyy-MM-dd date -> "must be a date like 2030-01-31"</item>
    /// <item>tags (optional), every violated rule adds its message: more than 10 -> "at most 10 tags";
    ///   an empty or over-30-character tag -> "each tag must be 1-30 characters"; an upper-case tag -> "every tag must be lowercase";
    ///   duplicates -> "tags must be unique"</item>
    /// <item>owner_id (optional): below 1 -> "must be positive"</item>
    /// </list>
    /// </summary>
    public static IReadOnlyDictionary<string, string[]> Validate(CreateTaskRequest request)
    {
        return new Dictionary<string, string[]>(); // TODO: apply the rules
    }
}

public static class TaskEndpoints
{
    /// <summary>
    /// POST /tasks: 422 application/problem+json with the validator's errors, or 201 with a Location header.
    /// Malformed JSON never reaches this code (the framework answers 400).
    /// </summary>
    public static void MapCreateTask(IEndpointRouteBuilder routes)
    {
        var lastId = 0;
        routes.MapPost("/tasks", (CreateTaskRequest request) =>
        {
            var errors = TaskValidator.Validate(request);
            if (errors.Count > 0)
            {
                return Results.ValidationProblem(errors.ToDictionary(e => e.Key, e => e.Value), statusCode: StatusCodes.Status422UnprocessableEntity);
            }

            var id = Interlocked.Increment(ref lastId);
            return Results.Created($"/tasks/{id}", new { id, title = request.Title!.Trim() });
        });
    }
}
