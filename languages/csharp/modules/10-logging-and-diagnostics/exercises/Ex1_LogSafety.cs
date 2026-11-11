namespace M10.Exercises;

public static class LogSafety
{
    /// <summary>
    /// Decides which correlation id a request gets. A caller-supplied id is only trusted if, after trimming
    /// whitespace, it is 1-64 characters long and consists solely of ASCII letters, digits, '-', '_' or '.'
    /// (anything else could forge log lines or smuggle headers). Otherwise return <paramref name="generate"/>().
    /// </summary>
    public static string ResolveCorrelationId(string? incoming, Func<string> generate)
    {
        throw new NotImplementedException();
    }

    /// <summary>
    /// Makes a query string safe to log. Parameters whose name (case-insensitive) is one of
    /// token, access_token, password, secret, api_key, apikey or authorization get the value replaced by <c>***</c>.
    /// Everything else, the order, a leading '?', and parameters without '=' stay exactly as they were.
    /// Null or empty input gives an empty string. Values are never decoded.
    /// </summary>
    public static string RedactQuery(string? query)
    {
        throw new NotImplementedException();
    }
}
