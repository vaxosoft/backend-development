namespace M14.Exercises;

public sealed record LintFinding(string Rule, int Line, string Message);

/// <summary>
/// A tiny Dockerfile checker over plain text. Lines are 1-based and refer to the first physical line of an
/// instruction (a trailing backslash continues an instruction; comments and blank lines are ignored;
/// instruction names are case-insensitive). Findings are ordered by line, then by rule id.
/// <list type="table">
/// <item><term>DF001</term><description>a FROM image without a tag/digest or tagged <c>latest</c> (ignore <c>scratch</c>, earlier stage names and images containing <c>$</c>); reported on that FROM</description></item>
/// <item><term>DF002</term><description>the final stage's image contains <c>/sdk</c> (SDK images do not belong in the runtime stage); reported on the final FROM</description></item>
/// <item><term>DF003</term><description>the final stage has no USER, or its last USER is root/0 (also <c>root:group</c>); reported on the final FROM</description></item>
/// <item><term>DF004</term><description>a COPY without <c>--from</c> whose source is <c>bin</c>, <c>obj</c> or below them (with or without <c>./</c>): build output must not come from the host; reported on the COPY</description></item>
/// <item><term>DF005</term><description>the final stage has no ENTRYPOINT/CMD (reported on the final FROM) or one in shell form, not a JSON array (reported on it)</description></item>
/// <item><term>DF006</term><description>an ADD whose source is not an http(s) URL (use COPY); reported on the ADD</description></item>
/// </list>
/// </summary>
public static class Ex3_DockerfileLinter
{
    public static IReadOnlyList<LintFinding> Lint(string dockerfile) => throw new NotImplementedException();
}
