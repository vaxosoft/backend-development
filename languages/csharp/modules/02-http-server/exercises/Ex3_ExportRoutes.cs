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
    public static void Map(IEndpointRouteBuilder routes)
    {
        throw new NotImplementedException();
    }
}
