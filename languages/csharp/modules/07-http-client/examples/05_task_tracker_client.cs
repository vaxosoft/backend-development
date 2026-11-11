using System.Net;
using System.Net.Http.Json;
using System.Text.Json;
using Microsoft.AspNetCore.Mvc;
using Microsoft.Extensions.DependencyInjection;

namespace M07.Examples;

/// <summary>Example 5: a typed Task Tracker API client, run against a real loopback server and a fake handler.</summary>
public static class TaskTrackerClientDemo
{
    public static async Task<int> RunAsync(string[] args)
    {
        await using var server = await LoopbackServer.StartAsync(MapTaskTrackerApi);

        var services = new ServiceCollection();
        services.AddHttpClient<TaskTrackerClient>(c => c.BaseAddress = server.BaseAddress);
        await using var provider = services.BuildServiceProvider();
        var client = provider.GetRequiredService<TaskTrackerClient>();

        var created = await client.CreateAsync(new NewTask("Write docs", new DateOnly(2030, 1, 31), ["docs"]));
        Console.WriteLine($"created #{created.Id}: {created.Title} ({created.Status})");
        Console.WriteLine($"get #1  -> {(await client.GetAsync(1))?.Title}");
        Console.WriteLine($"get #99 -> {(await client.GetAsync(99))?.Title ?? "null (404 mapped to null)"}");

        try
        {
            await client.CreateAsync(new NewTask("  "));
        }
        catch (TaskTrackerException ex)
        {
            Console.WriteLine($"invalid -> {(int)ex.Status}: {ex.Message}");
        }

        await client.CreateAsync(new NewTask("Ship it"));
        await foreach (var line in client.StreamExportAsync())
        {
            Console.WriteLine($"export line: {line}");
        }

        // The same client class runs against a stub: no sockets, no server, fully deterministic.
        var stubbed = new TaskTrackerClient(new HttpClient(new StubHandler(_ => new HttpResponseMessage(HttpStatusCode.NotFound)))
        {
            BaseAddress = new Uri("http://stub.invalid/"),
        });
        Console.WriteLine($"stubbed 404 -> {(await stubbed.GetAsync(1)) is null}");
        return 0;
    }

    private static void MapTaskTrackerApi(WebApplication app)
    {
        var tasks = new List<TaskItem>();

        app.MapPost("/tasks", (NewTask input) =>
        {
            if (string.IsNullOrWhiteSpace(input.Title))
            {
                return Results.Problem(title: "title must be 1-200 characters", statusCode: StatusCodes.Status422UnprocessableEntity);
            }

            var task = new TaskItem(tasks.Count + 1, input.Title.Trim(), TaskState.Todo, input.DueDate, input.Tags ?? [], null,
                new DateTimeOffset(2030, 1, 1, 9, 0, 0, TimeSpan.Zero));
            tasks.Add(task);
            return Results.Created($"/tasks/{task.Id}", task);
        });

        app.MapGet("/tasks/{id:int}", (int id) =>
            tasks.FirstOrDefault(t => t.Id == id) is { } task ? Results.Ok(task) : Results.NotFound());

        app.MapGet("/tasks/export", async (HttpResponse response) =>
        {
            response.ContentType = "application/x-ndjson";
            foreach (var task in tasks)
            {
                await response.WriteAsync(JsonSerializer.Serialize(task, Json.Options) + "\n");
            }
        });
    }
}

public sealed class TaskTrackerException(HttpStatusCode status, string message) : Exception(message)
{
    public HttpStatusCode Status { get; } = status;
}

public sealed class TaskTrackerClient(HttpClient http)
{
    public async Task<TaskItem> CreateAsync(NewTask input, CancellationToken cancellationToken = default)
    {
        using var response = await http.PostAsJsonAsync("tasks", input, Json.Options, cancellationToken);
        await ThrowIfFailedAsync(response, cancellationToken);
        return (await response.Content.ReadFromJsonAsync<TaskItem>(Json.Options, cancellationToken))!;
    }

    /// <summary>Returns null for 404: "not found" is an expected outcome, not an exception.</summary>
    public async Task<TaskItem?> GetAsync(int id, CancellationToken cancellationToken = default)
    {
        using var response = await http.GetAsync($"tasks/{id}", cancellationToken);
        if (response.StatusCode == HttpStatusCode.NotFound)
        {
            return null;
        }

        await ThrowIfFailedAsync(response, cancellationToken);
        return await response.Content.ReadFromJsonAsync<TaskItem>(Json.Options, cancellationToken);
    }

    /// <summary>Streams the NDJSON export line by line without buffering the whole body.</summary>
    public async IAsyncEnumerable<string> StreamExportAsync(
        [System.Runtime.CompilerServices.EnumeratorCancellation] CancellationToken cancellationToken = default)
    {
        using var response = await http.GetAsync("tasks/export", HttpCompletionOption.ResponseHeadersRead, cancellationToken);
        await ThrowIfFailedAsync(response, cancellationToken);
        await using var stream = await response.Content.ReadAsStreamAsync(cancellationToken);
        using var reader = new StreamReader(stream);
        while (await reader.ReadLineAsync(cancellationToken) is { } line)
        {
            yield return line;
        }
    }

    private static async Task ThrowIfFailedAsync(HttpResponseMessage response, CancellationToken cancellationToken)
    {
        if (response.IsSuccessStatusCode)
        {
            return;
        }

        var problem = response.Content.Headers.ContentType?.MediaType == "application/problem+json"
            ? await response.Content.ReadFromJsonAsync<ProblemDetails>(cancellationToken)
            : null;
        throw new TaskTrackerException(response.StatusCode, problem?.Title ?? response.ReasonPhrase ?? "request failed");
    }
}

/// <summary>A fake transport: HttpClient only needs an HttpMessageHandler.</summary>
public sealed class StubHandler(Func<HttpRequestMessage, HttpResponseMessage> respond) : HttpMessageHandler
{
    protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken) =>
        Task.FromResult(respond(request));
}
