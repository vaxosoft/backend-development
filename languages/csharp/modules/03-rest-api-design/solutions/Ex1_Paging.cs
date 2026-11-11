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
        request = default;
        error = limit < 1 ? "limit must be at least 1" : offset < 0 ? "offset must not be negative" : null;
        if (error is not null)
        {
            return false;
        }

        request = new PageRequest(Math.Min(limit ?? DefaultLimit, MaxLimit), offset ?? 0);
        return true;
    }
}

/// <summary>One page of a collection, serialized as {"items":[...],"total":N,"limit":L,"offset":O}.</summary>
public sealed record Page<T>(IReadOnlyList<T> Items, int Total, int Limit, int Offset)
{
    /// <summary>Slices <paramref name="source"/> (enumerated once); Total is the size of the whole source.</summary>
    public static Page<T> Of(IEnumerable<T> source, PageRequest request)
    {
        var all = source.ToList();
        return new Page<T>([.. all.Skip(request.Offset).Take(request.Limit)], all.Count, request.Limit, request.Offset);
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
        var text = string.IsNullOrEmpty(raw) ? defaultSort : raw;
        var descending = text.StartsWith('-');
        var field = descending ? text[1..] : text;

        spec = new SortSpec(field, descending);
        return allowed.Contains(field);
    }
}
