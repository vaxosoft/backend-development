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
    private sealed record Instruction(int Line, string Keyword, string Args);

    public static IReadOnlyList<LintFinding> Lint(string dockerfile)
    {
        var instructions = Parse(dockerfile);
        var findings = new List<LintFinding>();
        var stages = new HashSet<string>(StringComparer.OrdinalIgnoreCase);

        foreach (var from in instructions.Where(i => i.Keyword == "FROM"))
        {
            var (image, alias) = ParseFrom(from.Args);
            if (!stages.Contains(image) && image != "scratch" && !image.Contains('$') && IsUnpinned(image, out var why))
            {
                findings.Add(new LintFinding("DF001", from.Line, $"base image '{image}' {why}"));
            }

            if (alias is not null)
            {
                stages.Add(alias);
            }
        }

        var lastFrom = instructions.FindLastIndex(i => i.Keyword == "FROM");
        if (lastFrom >= 0)
        {
            var final = instructions[lastFrom];
            var finalStage = instructions.Skip(lastFrom + 1).ToList();

            if (ParseFrom(final.Args).Image.Contains("/sdk", StringComparison.OrdinalIgnoreCase))
            {
                findings.Add(new LintFinding("DF002", final.Line, "the final stage must not be an SDK image"));
            }

            var user = finalStage.LastOrDefault(i => i.Keyword == "USER")?.Args.Split(':')[0].Trim();
            if (user is null or "root" or "0")
            {
                findings.Add(new LintFinding("DF003", final.Line, "the final stage must run as a non-root USER"));
            }

            var start = finalStage.LastOrDefault(i => i.Keyword is "ENTRYPOINT" or "CMD");
            if (start is null)
            {
                findings.Add(new LintFinding("DF005", final.Line, "the final stage needs an ENTRYPOINT or CMD"));
            }
            else if (!start.Args.StartsWith('['))
            {
                findings.Add(new LintFinding("DF005", start.Line, $"{start.Keyword} must use exec form (JSON array)"));
            }
        }

        foreach (var copy in instructions.Where(i => i.Keyword == "COPY"))
        {
            var words = copy.Args.Split(' ', StringSplitOptions.RemoveEmptyEntries);
            if (words.Any(w => w.StartsWith("--from", StringComparison.Ordinal)))
            {
                continue;
            }

            var sources = words.Where(w => !w.StartsWith("--", StringComparison.Ordinal)).SkipLast(1);
            if (sources.Any(IsBuildOutput))
            {
                findings.Add(new LintFinding("DF004", copy.Line, "do not COPY bin/ or obj/ from the host; build inside the image"));
            }
        }

        foreach (var add in instructions.Where(i => i.Keyword == "ADD"))
        {
            var source = add.Args.Split(' ', StringSplitOptions.RemoveEmptyEntries).FirstOrDefault(w => !w.StartsWith("--", StringComparison.Ordinal));
            if (source is not null && !source.StartsWith("http://", StringComparison.OrdinalIgnoreCase) && !source.StartsWith("https://", StringComparison.OrdinalIgnoreCase))
            {
                findings.Add(new LintFinding("DF006", add.Line, "use COPY for local files"));
            }
        }

        return findings.OrderBy(f => f.Line).ThenBy(f => f.Rule, StringComparer.Ordinal).ToList();
    }

    private static bool IsBuildOutput(string source)
    {
        var path = source.StartsWith("./", StringComparison.Ordinal) ? source[2..] : source;
        return path is "bin" or "obj" || path.StartsWith("bin/", StringComparison.Ordinal) || path.StartsWith("obj/", StringComparison.Ordinal);
    }

    private static (string Image, string? Alias) ParseFrom(string args)
    {
        var words = args.Split(' ', StringSplitOptions.RemoveEmptyEntries).Where(w => !w.StartsWith("--", StringComparison.Ordinal)).ToList();
        var alias = words.Count >= 3 && words[1].Equals("AS", StringComparison.OrdinalIgnoreCase) ? words[2] : null;
        return (words.FirstOrDefault() ?? "", alias);
    }

    private static bool IsUnpinned(string image, out string why)
    {
        why = "";
        if (image.Contains('@'))
        {
            return false;
        }

        var name = image[(image.LastIndexOf('/') + 1)..];
        var colon = name.IndexOf(':');
        if (colon < 0)
        {
            why = "has no tag";
            return true;
        }

        why = "uses the moving tag 'latest'";
        return name[(colon + 1)..] == "latest";
    }

    private static List<Instruction> Parse(string text)
    {
        var result = new List<Instruction>();
        var lines = text.Replace("\r\n", "\n").Split('\n');
        for (var i = 0; i < lines.Length; i++)
        {
            var line = lines[i].Trim();
            if (line.Length == 0 || line.StartsWith('#'))
            {
                continue;
            }

            var start = i + 1;
            var joined = new System.Text.StringBuilder();
            while (true)
            {
                var continues = line.EndsWith('\\');
                joined.Append(continues ? line[..^1] : line).Append(' ');
                if (!continues || i + 1 >= lines.Length)
                {
                    break;
                }

                line = lines[++i].Trim();
                while (line.StartsWith('#') || (line.Length == 0 && i + 1 < lines.Length))
                {
                    line = lines[++i].Trim();
                }
            }

            var full = joined.ToString().Trim();
            var space = full.IndexOf(' ');
            var keyword = (space < 0 ? full : full[..space]).ToUpperInvariant();
            result.Add(new Instruction(start, keyword, space < 0 ? "" : full[(space + 1)..].Trim()));
        }

        return result;
    }
}
