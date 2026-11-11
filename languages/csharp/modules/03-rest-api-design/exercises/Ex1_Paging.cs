namespace M03.Exercises;

/// <summary>A validated page request: <c>limit</c> items starting at <c>offset</c>.</summary>
public readonly record struct PageRequest(int Limit, int Offset)
{
    public const int DefaultLimit = 20;
    public const int MaxLimit = 100;

    /// <summary>
    /// Missing limit -> 20, a limit above 100 is clamped to 100 (not an error), a limit below 1 is an error.
    /// Missing offset -> 0, a negative offset is an error. The error text mentions "limit" or "offset".
    /// </summary>
    public static bool TryCreate(int? limit, int? offset, out PageRequest request, out string? error)
    {
        throw new NotImplementedException();
    }
}

/// <summary>One page of a collection, serialized as {"items":[...],"total":N,"limit":L,"offset":O}.</summary>
public sealed record Page<T>(IReadOnlyList<T> Items, int Total, int Limit, int Offset)
{
    /// <summary>Slices <paramref name="source"/> (enumerated once); Total is the size of the whole source.</summary>
    public static Page<T> Of(IEnumerable<T> source, PageRequest request)
    {
        throw new NotImplementedException();
    }
}

/// <summary>A sort parameter such as "due_date" (ascending) or "-due_date" (descending).</summary>
public readonly record struct SortSpec(string Field, bool Descending)
{
    /// <summary>
    /// Null or empty -> <paramref name="defaultSort"/> (parsed the same way). A leading '-' means descending.
    /// The field must be in <paramref name="allowed"/>, otherwise the parse fails (never sort by arbitrary input).
    /// </summary>
    public static bool TryParse(string? raw, string defaultSort, IReadOnlySet<string> allowed, out SortSpec spec)
    {
        throw new NotImplementedException();
    }
}
