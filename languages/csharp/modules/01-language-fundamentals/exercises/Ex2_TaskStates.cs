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
        throw new NotImplementedException();
    }

    /// <summary>The wire name of a state; undefined enum values throw <see cref="ArgumentOutOfRangeException"/>.</summary>
    public static string ToWire(TaskState state)
    {
        throw new NotImplementedException();
    }

    /// <summary>
    /// Staying in the same state is allowed. Todo can go to InProgress or Done,
    /// InProgress to Todo or Done, and Done can only be reopened to Todo.
    /// </summary>
    public static bool CanTransition(TaskState from, TaskState to)
    {
        throw new NotImplementedException();
    }
}
