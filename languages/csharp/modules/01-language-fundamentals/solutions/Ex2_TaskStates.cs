namespace M01.Exercises;

/// <summary>Lifecycle states of a task.</summary>
public enum TaskState
{
    Todo,
    InProgress,
    Done,
}

/// <summary>Wire format and lifecycle rules for <see cref="TaskState"/>.</summary>
public static class TaskStates
{
    /// <summary>Accepts exactly "todo", "in_progress" and "done" (no numbers, no other casing).</summary>
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

    /// <summary>The wire name of a state; undefined enum values throw <see cref="ArgumentOutOfRangeException"/>.</summary>
    public static string ToWire(TaskState state) => state switch
    {
        TaskState.Todo => "todo",
        TaskState.InProgress => "in_progress",
        TaskState.Done => "done",
        _ => throw new ArgumentOutOfRangeException(nameof(state), state, null),
    };

    /// <summary>
    /// Staying in the same state is allowed. Todo can go to InProgress or Done,
    /// InProgress to Todo or Done, and Done can only be reopened to Todo.
    /// </summary>
    public static bool CanTransition(TaskState from, TaskState to) => (from, to) switch
    {
        (TaskState.Done, var target) => target is TaskState.Todo or TaskState.Done,
        _ => true,
    };
}
