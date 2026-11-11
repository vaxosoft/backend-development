using M12.TaskApi;

namespace M12.Exercises;

/// <summary>Pure validation rules for new tasks: easy to cover with a table of cases.</summary>
public static class Ex1_TaskValidator
{
    /// <summary>
    /// Field name -> message. Empty when valid. Rules:
    /// <c>title</c>: trimmed length 1-200 ("must be 1-200 characters");
    /// <c>due_date</c>: not before <paramref name="today"/> ("must not be in the past").
    /// </summary>
    public static IReadOnlyDictionary<string, string> Validate(NewTask task, DateOnly today)
    {
        var errors = new Dictionary<string, string>();

        if (task.Title?.Trim().Length is not (>= 1 and <= 200))
        {
            errors["title"] = "must be 1-200 characters";
        }

        if (task.DueDate < today)
        {
            errors["due_date"] = "must not be in the past";
        }

        return errors;
    }
}
