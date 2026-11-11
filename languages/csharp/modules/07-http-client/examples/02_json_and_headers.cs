using System.Net.Http.Headers;
using System.Net.Http.Json;

namespace M07.Examples;

/// <summary>Example 2: JSON round-trips and per-request headers (Bearer token).</summary>
public static class JsonAndHeaders
{
    public static async Task<int> RunAsync(string[] args)
    {
        var nextId = 0;
        await using var server = await LoopbackServer.StartAsync(app =>
        {
            app.MapPost("/tasks", (NewTask input) => Results.Json(
                new TaskItem(++nextId, input.Title, TaskState.Todo, input.DueDate, input.Tags ?? [], null,
                    new DateTimeOffset(2030, 1, 1, 9, 0, 0, TimeSpan.Zero)),
                statusCode: StatusCodes.Status201Created));

            app.MapGet("/me", (HttpRequest request) =>
                request.Headers.Authorization == "Bearer s3cret"
                    ? Results.Ok(new { name = "Ada", accept = request.Headers.Accept.ToString() })
                    : Results.Unauthorized());
        });

        using var client = new HttpClient { BaseAddress = server.BaseAddress };

        var created = await client.PostAsJsonAsync("tasks", new NewTask("Write docs", new DateOnly(2030, 1, 31), ["docs"]), Json.Options);
        Console.WriteLine($"POST /tasks -> {(int)created.StatusCode}");
        var task = await created.Content.ReadFromJsonAsync<TaskItem>(Json.Options);
        Console.WriteLine($"decoded     -> #{task!.Id} '{task.Title}' {task.Status} due {task.DueDate:yyyy-MM-dd} tags=[{string.Join(',', task.Tags)}]");
        using var second = await client.PostAsJsonAsync("tasks", new NewTask("Second"), Json.Options);
        Console.WriteLine($"raw body    -> {await second.Content.ReadAsStringAsync()}");

        // Authentication belongs on the request (or on a client that is dedicated to one credential).
        using var anonymous = await client.GetAsync("me");
        Console.WriteLine($"GET /me without token -> {(int)anonymous.StatusCode}");

        using var request = new HttpRequestMessage(HttpMethod.Get, "me");
        request.Headers.Authorization = new AuthenticationHeaderValue("Bearer", "s3cret");
        request.Headers.Accept.Add(new MediaTypeWithQualityHeaderValue("application/json"));
        using var response = await client.SendAsync(request);
        Console.WriteLine($"GET /me with token    -> {(int)response.StatusCode} {await response.Content.ReadAsStringAsync()}");

        return 0;
    }
}
