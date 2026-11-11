using Microsoft.Net.Http.Headers;

namespace M02.Exercises;

/// <summary>Serves the same task list as JSON, CSV or plain text depending on the Accept header.</summary>
public static class ExportRoutes
{
    public record Row(int Id, string Title, string Status);

    public static readonly IReadOnlyList<Row> Rows =
    [
        new(1, "Write docs", "todo"),
        new(2, "Fix login", "in_progress"),
    ];

    /// <summary>
    /// GET /tasks/export negotiates between (in server preference order) application/json,
    /// text/csv and text/plain:
    /// <list type="bullet">
    /// <item>no Accept header or */* -> application/json: [{"id":1,"title":"Write docs","status":"todo"},...] (camelCase keys)</item>
    /// <item>text/csv -> header line "id,title,status" then one "1,Write docs,todo" line per row, each ending in \n</item>
    /// <item>text/plain -> one "1. Write docs [todo]" line per row, each ending in \n</item>
    /// <item>the highest q-value wins; q=0 excludes a type; a specific range beats type/* beats */*; ties go to server order</item>
    /// <item>nothing acceptable -> 406</item>
    /// </list>
    /// Every response (including 406) carries "Vary: Accept".
    /// </summary>
    static readonly string[] Supported = ["application/json", "text/csv", "text/plain"];

    public static void Map(IEndpointRouteBuilder routes) =>
        routes.MapGet("/tasks/export", (HttpContext context) =>
        {
            context.Response.Headers.Append(HeaderNames.Vary, "Accept");
            return Choose(context.Request.GetTypedHeaders().Accept) switch
            {
                "application/json" => Results.Json(Rows),
                "text/csv" => Results.Text(Csv(), "text/csv"),
                "text/plain" => Results.Text(Plain(), "text/plain"),
                _ => Results.StatusCode(StatusCodes.Status406NotAcceptable),
            };
        });

    static string Csv() =>
        "id,title,status\n" + string.Concat(Rows.Select(r => $"{r.Id},{r.Title},{r.Status}\n"));

    static string Plain() => string.Concat(Rows.Select(r => $"{r.Id}. {r.Title} [{r.Status}]\n"));

    static string? Choose(IList<MediaTypeHeaderValue> accept)
    {
        if (accept.Count == 0)
        {
            return Supported[0];
        }

        string? best = null;
        var bestQuality = 0.0;
        foreach (var candidate in Supported)
        {
            var quality = QualityFor(candidate, accept);
            if (quality > bestQuality) // strictly greater: ties keep the server's preference order
            {
                best = candidate;
                bestQuality = quality;
            }
        }

        return best;
    }

    // The most specific matching range decides the quality of a candidate.
    static double QualityFor(string candidate, IList<MediaTypeHeaderValue> accept)
    {
        var specificity = -1;
        var quality = 0.0;
        var candidateType = new MediaTypeHeaderValue(candidate);
        foreach (var range in accept)
        {
            var rank = range switch
            {
                { MatchesAllTypes: true } => 0,
                { MatchesAllSubTypes: true } when range.Type == candidateType.Type => 1,
                _ when range.MediaType == candidateType.MediaType => 2,
                _ => -1,
            };
            if (rank > specificity)
            {
                specificity = rank;
                quality = range.Quality ?? 1.0;
            }
        }

        return quality;
    }
}
