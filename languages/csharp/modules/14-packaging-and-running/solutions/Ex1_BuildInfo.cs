namespace M14.Exercises;

/// <summary>
/// The version a binary reports about itself, parsed from the assembly's informational version
/// (<c>MAJOR.MINOR.PATCH[-prerelease][+commit]</c>, for example <c>1.4.2-rc.1+abc1234def</c>).
/// </summary>
public sealed record Ex1_BuildInfo(string Version, string? PreRelease, string? Commit)
{
    private const int ShortCommitLength = 7;

    public bool IsPreRelease => PreRelease is not null;

    /// <summary>
    /// Version must be three dot-separated non-negative integers. The pre-release label follows '-', the commit
    /// follows '+' and is shortened to at most 7 characters; an empty label or commit means none.
    /// Anything else throws <see cref="FormatException"/>.
    /// </summary>
    public static Ex1_BuildInfo Parse(string informationalVersion)
    {
        var text = informationalVersion;
        string? commit = null;
        string? preRelease = null;

        if (text.IndexOf('+') is var plus and >= 0)
        {
            commit = text[(plus + 1)..];
            text = text[..plus];
        }

        if (text.IndexOf('-') is var dash and >= 0)
        {
            preRelease = text[(dash + 1)..];
            text = text[..dash];
            if (preRelease.Length == 0)
            {
                throw new FormatException($"empty pre-release label in '{informationalVersion}'");
            }
        }

        var parts = text.Split('.');
        if (parts.Length != 3 || !parts.All(p => p.Length > 0 && p.All(char.IsAsciiDigit)))
        {
            throw new FormatException($"'{informationalVersion}' is not MAJOR.MINOR.PATCH");
        }

        commit = string.IsNullOrEmpty(commit) ? null : commit[..Math.Min(commit.Length, ShortCommitLength)];
        return new Ex1_BuildInfo(text, preRelease, commit);
    }

    /// <summary><c>1.4.2</c>, <c>1.4.2-rc.1</c>, <c>1.4.2 (abc1234)</c> or <c>1.4.2-rc.1 (abc1234)</c>.</summary>
    public string ToDisplayString()
    {
        var core = PreRelease is null ? Version : $"{Version}-{PreRelease}";
        return Commit is null ? core : $"{core} ({Commit})";
    }
}
