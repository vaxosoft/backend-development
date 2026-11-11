namespace M02.Examples;

/// <summary>The smallest Minimal API: routes, route parameters, query parameters, route constraints.</summary>
static class Hello
{
    public static async Task<int> RunAsync(string[] args)
    {
        var builder = ExampleHost.CreateBuilder(args);
        var app = builder.Build();

        app.MapGet("/", () => "Hello, world!");
        app.MapGet("/hello/{name}", (string name) => $"Hello, {name}!");
        app.MapGet("/greet", (string? who, int times = 1) =>
            string.Join(' ', Enumerable.Repeat($"Hi {who ?? "you"}!", times)));
        app.MapGet("/users/{id:int}", (int id) => Results.Ok(new { id }));
        app.MapPost("/ping", () => Results.NoContent());

        await app.RunAsync();
        return 0;
    }
}
