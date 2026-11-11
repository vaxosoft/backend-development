namespace P2.TasksCrudApi;

public static class TaskValidator
{
    public static readonly IReadOnlyList<string> Statuses = ["todo", "in_progress", "done"];

    /// <summary>Checks a POST/PUT body against the Task rules and normalises it (trimmed title, default status).</summary>
    /// <returns>The data when valid, otherwise field name -> messages (keys are the JSON names, e.g. <c>due_date</c>).</returns>
    public static (TaskData? Data, Dictionary<string, string[]> Errors) Validate(TaskRequest request) =>
        throw new NotImplementedException("TODO: title 1-200 (trimmed), status, due_date YYYY-MM-DD, tags rules");

    /// <summary>Applies a PATCH to an existing task and validates the result.</summary>
    public static (TaskData? Data, Dictionary<string, string[]> Errors) Apply(TaskItem existing, TaskPatch patch) =>
        throw new NotImplementedException("TODO: absent = unchanged, explicit null due_date = cleared");
}
