using System.Globalization;

namespace P2.TasksCrudApi;

public static class TaskValidator
{
    private const string DateFormat = "yyyy-MM-dd";

    public static readonly IReadOnlyList<string> Statuses = ["todo", "in_progress", "done"];

    /// <summary>Checks a POST/PUT body against the Task rules and normalises it (trimmed title, default status).</summary>
    /// <returns>The data when valid, otherwise field name -> messages (keys are the JSON names, e.g. <c>due_date</c>).</returns>
    public static (TaskData? Data, Dictionary<string, string[]> Errors) Validate(TaskRequest request)
    {
        var errors = new Dictionary<string, string[]>();

        var title = (request.Title ?? "").Trim();
        if (title.Length is < 1 or > 200)
        {
            errors["title"] = ["must be 1-200 characters"];
        }

        var status = request.Status ?? "todo";
        if (!Statuses.Contains(status))
        {
            errors["status"] = [$"must be one of: {string.Join(", ", Statuses)}"];
        }

        DateOnly? dueDate = null;
        if (request.DueDate is not null)
        {
            if (DateOnly.TryParseExact(request.DueDate, DateFormat, CultureInfo.InvariantCulture, DateTimeStyles.None, out var parsed))
            {
                dueDate = parsed;
            }
            else
            {
                errors["due_date"] = ["must be a date formatted as YYYY-MM-DD"];
            }
        }

        var tags = request.Tags ?? [];
        if (TagProblem(tags) is { } problem)
        {
            errors["tags"] = [problem];
        }

        return errors.Count > 0
            ? (null, errors)
            : (new TaskData(title, status, dueDate, tags, request.OwnerId), errors);
    }

    /// <summary>Applies a PATCH to an existing task and validates the result.</summary>
    public static (TaskData? Data, Dictionary<string, string[]> Errors) Apply(TaskItem existing, TaskPatch patch) =>
        Validate(new TaskRequest(
            patch.HasTitle ? patch.Title : existing.Title,
            patch.HasStatus ? patch.Status ?? "" : existing.Status, // an explicit null status is invalid, not "default"
            patch.HasDueDate ? patch.DueDate : existing.DueDate?.ToString(DateFormat, CultureInfo.InvariantCulture),
            patch.HasTags ? patch.Tags : [.. existing.Tags],
            existing.OwnerId));

    private static string? TagProblem(List<string> tags)
    {
        if (tags.Count > 10)
        {
            return "must contain at most 10 tags";
        }

        if (tags.Any(tag => tag.Length is < 1 or > 30))
        {
            return "each tag must be 1-30 characters";
        }

        if (tags.Any(tag => tag != tag.ToLowerInvariant()))
        {
            return "tags must be lowercase";
        }

        return tags.Distinct().Count() != tags.Count ? "tags must be unique" : null;
    }
}
