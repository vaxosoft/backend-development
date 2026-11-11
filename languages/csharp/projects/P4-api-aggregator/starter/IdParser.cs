namespace P4.ApiAggregator;

public static class IdParser
{
    public const int MaxIds = 50;

    /// <summary>
    /// Parses the <c>ids</c> query value (<c>"1,2,3"</c>). Returns false with a human-readable
    /// <paramref name="error"/> when it is missing/empty, has more than <see cref="MaxIds"/> entries,
    /// or contains anything that is not a non-negative integer. Order and duplicates are preserved.
    /// </summary>
    public static bool TryParse(string? raw, out IReadOnlyList<int> ids, out string error)
    {
        // TODO(P4): implement the rules described above.
        _ = raw;
        ids = [];
        error = "TODO";
        return true;
    }
}
