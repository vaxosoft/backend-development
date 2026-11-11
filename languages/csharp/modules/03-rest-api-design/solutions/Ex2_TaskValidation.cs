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
        var errors = new Dictionary<string, List<string>>();

        void Add(string field, string message)
        {
            if (!errors.TryGetValue(field, out var list))
            {
                errors[field] = list = [];
            }

            list.Add(message);
        }

        if (request.Title is null)
        {
            Add("title", "is required");
        }
        else if (request.Title.Trim().Length is < 1 or > 200)
        {
            Add("title", "must be 1-200 characters");
        }

        if (request.Status is not null and not ("todo" or "in_progress" or "done"))
        {
            Add("status", "must be one of: todo, in_progress, done");
        }

        if (request.DueDate is not null
            && !DateOnly.TryParseExact(request.DueDate, "yyyy-MM-dd", CultureInfo.InvariantCulture, DateTimeStyles.None, out _))
        {
            Add("due_date", "must be a date like 2030-01-31");
        }

        if (request.Tags is { } tags)
        {
            if (tags.Length > 10)
            {
                Add("tags", "at most 10 tags");
            }

            if (tags.Any(t => t.Length is < 1 or > 30))
            {
                Add("tags", "each tag must be 1-30 characters");
            }

            if (tags.Any(t => t != t.ToLowerInvariant()))
            {
                Add("tags", "every tag must be lowercase");
            }

            if (tags.Distinct().Count() != tags.Length)
            {
                Add("tags", "tags must be unique");
            }
        }

        if (request.OwnerId is < 1)
        {
            Add("owner_id", "must be positive");
        }

        return errors.ToDictionary(e => e.Key, e => e.Value.ToArray());
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
