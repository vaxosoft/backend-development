using System.Globalization;

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
        ids = [];
        if (string.IsNullOrWhiteSpace(raw))
        {
            error = "ids is required, e.g. ?ids=1,2,3";
            return false;
        }

        var parts = raw.Split(',', StringSplitOptions.TrimEntries);
        if (parts.Length > MaxIds)
        {
            error = $"at most {MaxIds} ids are allowed";
            return false;
        }

        var parsed = new List<int>(parts.Length);
        foreach (var part in parts)
        {
            if (!int.TryParse(part, NumberStyles.None, CultureInfo.InvariantCulture, out var id))
            {
                error = $"'{part}' is not a valid id";
                return false;
            }

            parsed.Add(id);
        }

        ids = parsed;
        error = "";
        return true;
    }
}
