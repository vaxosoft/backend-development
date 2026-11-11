using System.Net;
using System.Text.RegularExpressions;

namespace M13.Exercises;

/// <summary>Small text utilities used by the Task Tracker: slugs, hashtags and templates.</summary>
public static partial class Ex2_Text
{
    [GeneratedRegex("[^a-z0-9]+", RegexOptions.CultureInvariant, matchTimeoutMilliseconds: 1000)]
    private static partial Regex NonSlugRun();

    [GeneratedRegex(@"#(?<tag>[a-z0-9_-]+)", RegexOptions.IgnoreCase | RegexOptions.CultureInvariant, matchTimeoutMilliseconds: 1000)]
    private static partial Regex Hashtag();

    [GeneratedRegex(@"\{\{\s*(?<name>\w+)\s*\}\}", RegexOptions.CultureInvariant, matchTimeoutMilliseconds: 1000)]
    private static partial Regex Placeholder();

    /// <summary>
    /// Lower-case ASCII slug: every run of characters other than a-z, A-Z, 0-9 becomes one '-',
    /// with no leading or trailing '-'. "Hello,  World!" -> "hello-world".
    /// </summary>
    public static string Slugify(string text) =>
        NonSlugRun().Replace(text.ToLowerInvariant(), "-").Trim('-');

    /// <summary>
    /// Hashtags in the text: '#' followed by 1-30 characters of [a-z0-9_-] (case-insensitive match),
    /// returned lower-case, distinct, in order of first appearance. Longer tags are ignored entirely.
    /// </summary>
    public static IReadOnlyList<string> ExtractTags(string text) =>
        Hashtag().Matches(text)
            .Select(m => m.Groups["tag"].Value.ToLowerInvariant())
            .Where(tag => tag.Length <= 30)
            .Distinct()
            .ToList();

    /// <summary>
    /// Replaces <c>{{name}}</c> (optional spaces inside the braces, e.g. <c>{{ name }}</c>) with the HTML-encoded
    /// value. A name without a value throws <see cref="KeyNotFoundException"/> naming the placeholder.
    /// </summary>
    public static string Render(string template, IReadOnlyDictionary<string, string> values) =>
        Placeholder().Replace(template, match =>
        {
            var name = match.Groups["name"].Value;
            return values.TryGetValue(name, out var value)
                ? WebUtility.HtmlEncode(value)
                : throw new KeyNotFoundException($"no value for placeholder '{name}'");
        });
}
