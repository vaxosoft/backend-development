using System.Globalization;
using System.Text.Json;

namespace M04.Exercises;

/// <summary>Result of applying a PATCH body: the new task, or a malformed-body flag, or per-field errors.</summary>
public sealed record PatchOutcome(TaskDto? Task, bool Malformed, IReadOnlyDictionary<string, string> Errors)
{
    public bool Ok => Task is not null;
}

public static class Ex3_Patch
{
    private static readonly Dictionary<string, string> NoErrors = [];

    /// <summary>
    /// Apply a JSON merge-style PATCH to <paramref name="current"/>.
    /// Only title, status, due_date and tags may appear. Absent = unchanged; due_date null clears it.
    /// Not JSON / not an object => Malformed (400). Otherwise collect every field error (422) and return no task:
    /// title trimmed 1-200 chars (null not allowed); status todo|in_progress|done; due_date YYYY-MM-DD or null;
    /// tags an array of at most 10 unique lower-case strings of 1-30 chars (null not allowed);
    /// any other member (id, owner_id, created_at, ...) is an error "unknown or read-only field".
    /// </summary>
    public static PatchOutcome Apply(TaskDto current, string json)
    {
        JsonDocument doc;
        try
        {
            doc = JsonDocument.Parse(json);
        }
        catch (JsonException)
        {
            return new PatchOutcome(null, true, NoErrors);
        }

        using (doc)
        {
            if (doc.RootElement.ValueKind != JsonValueKind.Object)
            {
                return new PatchOutcome(null, true, NoErrors);
            }

            var task = current;
            var errors = new Dictionary<string, string>();
            foreach (var member in doc.RootElement.EnumerateObject())
            {
                var value = member.Value;
                switch (member.Name)
                {
                    case "title":
                        if (value.ValueKind == JsonValueKind.String && value.GetString()!.Trim() is { Length: >= 1 and <= 200 } title)
                        {
                            task = task with { Title = title };
                        }
                        else
                        {
                            errors["title"] = "must be 1-200 characters";
                        }

                        break;
                    case "status":
                        if (value.ValueKind == JsonValueKind.String && ParseStatus(value.GetString()!) is { } status)
                        {
                            task = task with { Status = status };
                        }
                        else
                        {
                            errors["status"] = "must be one of todo, in_progress, done";
                        }

                        break;
                    case "due_date":
                        if (value.ValueKind == JsonValueKind.Null)
                        {
                            task = task with { DueDate = null };
                        }
                        else if (value.ValueKind == JsonValueKind.String &&
                                 DateOnly.TryParseExact(value.GetString(), "yyyy-MM-dd", CultureInfo.InvariantCulture, DateTimeStyles.None, out var due))
                        {
                            task = task with { DueDate = due };
                        }
                        else
                        {
                            errors["due_date"] = "must be YYYY-MM-DD or null";
                        }

                        break;
                    case "tags":
                        if (ParseTags(value) is { } tags)
                        {
                            task = task with { Tags = tags };
                        }
                        else
                        {
                            errors["tags"] = "must be at most 10 unique lowercase strings of 1-30 characters";
                        }

                        break;
                    default:
                        errors[member.Name] = "unknown or read-only field";
                        break;
                }
            }

            return errors.Count > 0 ? new PatchOutcome(null, false, errors) : new PatchOutcome(task, false, NoErrors);
        }
    }

    private static Status? ParseStatus(string text) => text switch
    {
        "todo" => Status.Todo,
        "in_progress" => Status.InProgress,
        "done" => Status.Done,
        _ => null,
    };

    private static string[]? ParseTags(JsonElement value)
    {
        if (value.ValueKind != JsonValueKind.Array || value.GetArrayLength() > 10)
        {
            return null;
        }

        var tags = new List<string>();
        foreach (var item in value.EnumerateArray())
        {
            if (item.ValueKind != JsonValueKind.String ||
                item.GetString()! is not { Length: >= 1 and <= 30 } tag ||
                tag != tag.ToLowerInvariant() ||
                tags.Contains(tag))
            {
                return null;
            }

            tags.Add(tag);
        }

        return [.. tags];
    }
}
