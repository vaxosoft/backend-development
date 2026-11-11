namespace M06.Exercises;

/// <summary>Thread-safe counters per task status, shared by many request threads.</summary>
public sealed class Ex1_StatusCounter
{
    /// <summary>Add one to the status and return the new count. Must not lose updates under contention.</summary>
    public int Increment(string status) => throw new NotImplementedException();

    public int Get(string status) => throw new NotImplementedException();

    /// <summary>Sum of all counters.</summary>
    public int Total => throw new NotImplementedException();

    /// <summary>A copy: later increments must not change a snapshot already handed out.</summary>
    public IReadOnlyDictionary<string, int> Snapshot() => throw new NotImplementedException();

    /// <summary>
    /// Add one only if the status currently has fewer than <paramref name="limit"/>; return whether it was added.
    /// The check and the increment must be one atomic step (a work-in-progress limit).
    /// </summary>
    public bool TryIncrementBelow(string status, int limit) => throw new NotImplementedException();
}
