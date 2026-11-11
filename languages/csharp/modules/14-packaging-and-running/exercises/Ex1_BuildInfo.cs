namespace M14.Exercises;

/// <summary>
/// The version a binary reports about itself, parsed from the assembly's informational version
/// (<c>MAJOR.MINOR.PATCH[-prerelease][+commit]</c>, for example <c>1.4.2-rc.1+abc1234def</c>).
/// </summary>
public sealed record Ex1_BuildInfo(string Version, string? PreRelease, string? Commit)
{
    public bool IsPreRelease => PreRelease is not null;

    /// <summary>
    /// Version must be three dot-separated non-negative integers. The pre-release label follows '-', the commit
    /// follows '+' and is shortened to at most 7 characters; an empty label or commit means none.
    /// Anything else throws <see cref="FormatException"/>.
    /// </summary>
    public static Ex1_BuildInfo Parse(string informationalVersion) => throw new NotImplementedException();

    /// <summary><c>1.4.2</c>, <c>1.4.2-rc.1</c>, <c>1.4.2 (abc1234)</c> or <c>1.4.2-rc.1 (abc1234)</c>.</summary>
    public string ToDisplayString() => throw new NotImplementedException();
}
