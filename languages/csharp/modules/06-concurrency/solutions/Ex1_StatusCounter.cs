namespace M06.Exercises;

/// <summary>Thread-safe counters per task status, shared by many request threads.</summary>
public sealed class Ex1_StatusCounter
{
    private readonly object _gate = new();
    private readonly Dictionary<string, int> _counts = [];

    /// <summary>Add one to the status and return the new count. Must not lose updates under contention.</summary>
    public int Increment(string status)
    {
        lock (_gate)
        {
            return _counts[status] = _counts.GetValueOrDefault(status) + 1;
        }
    }

    public int Get(string status)
    {
        lock (_gate)
        {
            return _counts.GetValueOrDefault(status);
        }
    }

    /// <summary>Sum of all counters.</summary>
    public int Total
    {
        get
        {
            lock (_gate)
            {
                return _counts.Values.Sum();
            }
        }
    }

    /// <summary>A copy: later increments must not change a snapshot already handed out.</summary>
    public IReadOnlyDictionary<string, int> Snapshot()
    {
        lock (_gate)
        {
            return new Dictionary<string, int>(_counts);
        }
    }

    /// <summary>
    /// Add one only if the status currently has fewer than <paramref name="limit"/>; return whether it was added.
    /// The check and the increment must be one atomic step (a work-in-progress limit).
    /// </summary>
    public bool TryIncrementBelow(string status, int limit)
    {
        lock (_gate)
        {
            var current = _counts.GetValueOrDefault(status);
            if (current >= limit)
            {
                return false;
            }

            _counts[status] = current + 1;
            return true;
        }
    }
}
