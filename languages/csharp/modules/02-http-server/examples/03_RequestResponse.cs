using System.Net.Mime;
using Microsoft.AspNetCore.Mvc;

namespace M02.Examples;

/// <summary>Binding sources (route, query, header, body, form, file) and response writing.</summary>
static class RequestResponse
{
    record NewNote(string Title, string[]? Tags);

    public static async Task<int> RunAsync(string[] args)
    {
        var builder = ExampleHost.CreateBuilder(args);
        var app = builder.Build();

        // Route + query + header; every parameter is bound by name and type.
        app.MapGet("/notes/{id:int}", (int id, string? format, [FromHeader(Name = "X-Client")] string? client) =>
            Results.Ok(new { id, format = format ?? "full", client = client ?? "unknown" }));

        // JSON body -> record; malformed JSON becomes a 400 before the handler runs.
        app.MapPost("/notes", (NewNote note) =>
            TypedResults.Created($"/notes/1", new { id = 1, note.Title, tags = note.Tags ?? [] }));

        // Form fields need antiforgery by default; this demo opts out explicitly.
        app.MapPost("/form", ([FromForm] string name, [FromForm] int age) => $"{name} is {age}")
            .DisableAntiforgery();

        app.MapPost("/upload", async (IFormFile file) =>
        {
            await using var stream = file.OpenReadStream();
            using var reader = new StreamReader(stream);
            var text = await reader.ReadToEndAsync();
            return Results.Ok(new { file.FileName, file.Length, file.ContentType, lines = text.Split('\n', StringSplitOptions.RemoveEmptyEntries).Length });
        }).DisableAntiforgery();

        // Streaming: chunks are flushed as they are produced instead of buffering the whole body.
        app.MapGet("/stream", () => Results.Stream(async body =>
        {
            for (var i = 1; i <= 3; i++)
            {
                await body.WriteAsync(System.Text.Encoding.UTF8.GetBytes($"chunk {i}\n"));
                await body.FlushAsync();
                await Task.Delay(20);
            }
        }, MediaTypeNames.Text.Plain));

        // Cookies and custom headers.
        app.MapGet("/visit", (HttpContext context) =>
        {
            var seen = context.Request.Cookies.TryGetValue("visited", out var count) && int.TryParse(count, out var n) ? n : 0;
            context.Response.Cookies.Append("visited", (seen + 1).ToString(), new CookieOptions { HttpOnly = true, SameSite = SameSiteMode.Lax, MaxAge = TimeSpan.FromHours(1) });
            context.Response.Headers["Cache-Control"] = "no-store";
            return $"visit {seen + 1}";
        });

        // Content negotiation by hand: look at Accept and pick a representation.
        app.MapGet("/negotiate", (HttpContext context) =>
        {
            var accept = context.Request.Headers.Accept.ToString();
            return accept switch
            {
                _ when accept.Contains("text/plain") => Results.Text("plain text", MediaTypeNames.Text.Plain),
                "" or "*/*" => Results.Json(new { kind = "json" }),
                _ when accept.Contains("application/json") => Results.Json(new { kind = "json" }),
                _ => Results.StatusCode(StatusCodes.Status406NotAcceptable),
            };
        });

        await app.RunAsync();
        return 0;
    }
}
