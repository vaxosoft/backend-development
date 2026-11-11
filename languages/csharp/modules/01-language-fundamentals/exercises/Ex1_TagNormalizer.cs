namespace M01.Exercises;

/// <summary>Normalizes user-supplied tags for the Task Tracker.</summary>
public static class TagNormalizer
{
    /// <summary>
    /// Trims and lower-cases (invariant culture) every tag, drops null/blank entries and
    /// duplicates (the first occurrence wins, order is kept). The source is enumerated once.
    /// </summary>
    public static IReadOnlyList<string> Normalize(IEnumerable<string?> raw)
    {
        throw new NotImplementedException();
    }
}
