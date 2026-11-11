using System.Net;
using System.Text;
using System.Text.RegularExpressions;

namespace M13.Examples;

public static partial class RegexAndTemplates
{
    [GeneratedRegex(@"#(?<tag>[a-z0-9_-]{1,30})", RegexOptions.CultureInvariant, matchTimeoutMilliseconds: 200)]
    private static partial Regex TagPattern();

    [GeneratedRegex(@"\{\{(\w+)\}\}", RegexOptions.CultureInvariant, matchTimeoutMilliseconds: 200)]
    private static partial Regex Placeholder();

    private static readonly CompositeFormat Line = CompositeFormat.Parse("[{0,-11}] {1}");

    private static string Render(string template, IReadOnlyDictionary<string, string> values) =>
        Placeholder().Replace(template, m =>
            values.TryGetValue(m.Groups[1].Value, out var value)
                ? WebUtility.HtmlEncode(value)
                : throw new KeyNotFoundException($"no value for '{m.Groups[1].Value}'"));

    public static Task<int> RunAsync(string[] args)
    {
        var tags = TagPattern().Matches("Write docs #docs #p1 #Not_Lower").Select(m => m.Groups["tag"].Value);
        Console.WriteLine($"tags: {string.Join(",", tags)}");

        var hostile = new Regex(@"^(a+)+$", RegexOptions.None, TimeSpan.FromMilliseconds(100));
        var input = new string('a', 40) + "!";
        try
        {
            hostile.IsMatch(input);
        }
        catch (RegexMatchTimeoutException)
        {
            Console.WriteLine("backtracking engine: gave up after the 100 ms timeout");
        }

        var safe = new Regex(@"^(a+)+$", RegexOptions.NonBacktracking, TimeSpan.FromMilliseconds(100));
        Console.WriteLine($"NonBacktracking engine: matched={safe.IsMatch(input)}");

        var template = """
            Hello {{name}},
            you have {{count}} open tasks.
            """;
        Console.WriteLine(Render(template, new Dictionary<string, string> { ["name"] = "<Ada>", ["count"] = "3" }));
        try
        {
            Render(template, new Dictionary<string, string> { ["name"] = "Ada" });
        }
        catch (KeyNotFoundException ex)
        {
            Console.WriteLine($"missing value: {ex.Message}");
        }

        var task = "Write docs";
        var json = $$"""{"title":"{{task}}","status":"todo"}""";
        Console.WriteLine(json);
        Console.WriteLine(string.Format(null, Line, "in_progress", task));
        return Task.FromResult(0);
    }
}
