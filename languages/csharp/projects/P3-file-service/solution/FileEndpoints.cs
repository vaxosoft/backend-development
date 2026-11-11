using Microsoft.AspNetCore.Http.Features;
using Microsoft.AspNetCore.WebUtilities;
using Microsoft.Net.Http.Headers;

namespace P3.FileService;

public static class FileEndpoints
{
    // Room for the multipart boundaries and part headers around the file bytes.
    private const int MultipartOverhead = 64 * 1024;

    public static void MapFileEndpoints(this IEndpointRouteBuilder app)
    {
        var files = app.MapGroup("/files");
        files.MapPost("", UploadAsync);
        files.MapGet("/{id}", Download);
        files.MapDelete("/{id}", Delete);
    }

    private static async Task<IResult> UploadAsync(HttpRequest request, FileStore store, CancellationToken ct)
    {
        // Not IFormFile / ReadFormAsync: those buffer the whole upload before the handler sees it.
        if (!MediaTypeHeaderValue.TryParse(request.ContentType, out var media)
            || !media.MediaType.Equals("multipart/form-data", StringComparison.OrdinalIgnoreCase)
            || HeaderUtilities.RemoveQuotes(media.Boundary) is not { HasValue: true, Length: > 0 and <= 70 } boundary)
        {
            return BadRequest("expected a multipart/form-data body");
        }

        // Kestrel's global limit is 30 MB. Per request, the real limit plus the multipart overhead applies:
        // Kestrel then refuses to read more (and drops the connection instead of draining the rest).
        if (request.HttpContext.Features.Get<IHttpMaxRequestBodySizeFeature>() is { IsReadOnly: false } bodyLimit)
        {
            bodyLimit.MaxRequestBodySize = store.MaxUploadBytes + MultipartOverhead;
        }

        try
        {
            var reader = new MultipartReader(boundary.Value!, request.Body);
            while (await reader.ReadNextSectionAsync(ct) is { } section)
            {
                if (!ContentDispositionHeaderValue.TryParse(section.ContentDisposition, out var disposition)
                    || !disposition.IsFileDisposition()
                    || HeaderUtilities.RemoveQuotes(disposition.Name) != "file")
                {
                    continue;
                }

                var clientName = disposition.FileNameStar.HasValue ? disposition.FileNameStar.Value : HeaderUtilities.RemoveQuotes(disposition.FileName).Value;
                var stored = await store.SaveAsync(section.Body, clientName, section.ContentType, ct);
                return Results.Created($"/files/{stored.Id}", stored);
            }
        }
        catch (Exception ex) when (ex is FileTooLargeException or BadHttpRequestException { StatusCode: StatusCodes.Status413PayloadTooLarge })
        {
            return Results.Problem(
                statusCode: StatusCodes.Status413PayloadTooLarge,
                title: "File too large",
                detail: $"The limit is {store.MaxUploadBytes} bytes.");
        }
        catch (InvalidDataException)
        {
            return BadRequest("malformed multipart body");
        }

        return BadRequest("multipart field 'file' is required");
    }

    private static IResult Download(string id, HttpContext http, FileStore store)
    {
        if (store.Open(id) is not { } download)
        {
            return NotFound();
        }

        http.Response.Headers.ContentDisposition = AttachmentHeader(download.File.Name);

        // ETag + If-None-Match are handled by the result: a match answers 304 without a body.
        return Results.Stream(
            download.Content,
            download.File.ContentType,
            entityTag: new EntityTagHeaderValue($"\"{download.File.Sha256}\""));
    }

    /// <summary>ASCII fallback in <c>filename</c> plus the exact UTF-8 name in <c>filename*</c> (RFC 6266).</summary>
    private static string AttachmentHeader(string name)
    {
        var fallback = string.Concat(name.Select(c => c is < ' ' or > '~' or '"' or '\\' ? '_' : c));
        return $"attachment; filename=\"{fallback}\"; filename*=UTF-8''{Uri.EscapeDataString(name)}";
    }

    private static IResult Delete(string id, FileStore store) =>
        store.Delete(id) ? Results.NoContent() : NotFound();

    private static IResult NotFound() => Results.Problem(statusCode: StatusCodes.Status404NotFound, title: "File not found");

    private static IResult BadRequest(string detail) =>
        Results.Problem(statusCode: StatusCodes.Status400BadRequest, title: "Bad request", detail: detail);
}
