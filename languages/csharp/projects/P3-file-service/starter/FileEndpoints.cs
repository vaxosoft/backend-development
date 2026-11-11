namespace P3.FileService;

public static class FileEndpoints
{
    public static void MapFileEndpoints(this IEndpointRouteBuilder app)
    {
        var files = app.MapGroup("/files");
        files.MapPost("", UploadAsync);
        files.MapGet("/{id}", Download);
        files.MapDelete("/{id}", Delete);
    }

    private static Task<IResult> UploadAsync(HttpRequest request, FileStore store, CancellationToken ct) =>
        throw new NotImplementedException("TODO: read the multipart body as a stream, save part 'file', 201 + Location, 413, 400");

    private static IResult Download(string id, HttpContext http, FileStore store) =>
        throw new NotImplementedException("TODO: stream with Content-Length/Type/Disposition and ETag (304 on If-None-Match)");

    private static IResult Delete(string id, FileStore store) =>
        throw new NotImplementedException("TODO: 204 or 404");
}
