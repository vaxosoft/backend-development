namespace M03.Examples;

/// <summary>Declarative validation with DataAnnotations and AddValidation, ProblemDetails responses.</summary>
static class Validation
{
    public static async Task<int> RunAsync(string[] args)
    {
        var builder = ExampleHost.CreateBuilder(args);
        builder.Services.ConfigureHttpJsonOptions(o => Json.Configure(o.SerializerOptions));
        builder.Services.AddValidatedProblemDetails();

        var app = builder.Build();
        app.UseStatusCodePages();

        // The framework validates TaskInput before this lambda runs; on failure it never runs.
        app.MapPost("/tasks", (TaskInput input) => TypedResults.Ok(input));

        await app.RunAsync();
        return 0;
    }
}
