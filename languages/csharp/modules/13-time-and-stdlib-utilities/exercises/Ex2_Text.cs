namespace M13.Exercises;

/// <summary>Small text utilities used by the Task Tracker: slugs, hashtags and templates.</summary>
public static class Ex2_Text
{
    /// <summary>
    /// Lower-case ASCII slug: every run of characters other than a-z, A-Z, 0-9 becomes one '-',
    /// with no leading or trailing '-'. "Hello,  World!" -> "hello-world".
    /// </summary>
    public static string Slugify(string text) => throw new NotImplementedException();

    /// <summary>
    /// Hashtags in the text: '#' followed by 1-30 characters of [a-z0-9_-] (case-insensitive match),
    /// returned lower-case, distinct, in order of first appearance. Longer tags are ignored entirely.
    /// </summary>
    public static IReadOnlyList<string> ExtractTags(string text) => throw new NotImplementedException();

    /// <summary>
    /// Replaces <c>{{name}}</c> (optional spaces inside the braces, e.g. <c>{{ name }}</c>) with the HTML-encoded
    /// value. A name without a value throws <see cref="KeyNotFoundException"/> naming the placeholder.
    /// </summary>
    public static string Render(string template, IReadOnlyDictionary<string, string> values) => throw new NotImplementedException();
}
