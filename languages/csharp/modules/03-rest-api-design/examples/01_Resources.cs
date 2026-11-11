using Microsoft.AspNetCore.Http.HttpResults;

namespace M03.Examples;

/// <summary>Resource modelling: nouns, verbs and status codes, with a DTO boundary.</summary>
static class Resources
{
    static Results<Created<TaskItem>, ProblemHttpResult> Create(TaskInput input, TaskRepository repo)
    {
        if (string.IsNullOrWhiteSpace(input.Title))
        {
            return Problems.Invalid("title", "is required");
        }

        if (repo.All().Any(t => t.Title == input.Title.Trim()))
        {
            return Problems.Conflict("a task with this title already exists");
        }

        var task = repo.Add(input);
        return TypedResults.Created($"/tasks/{task.Id}", task);
    }

    static Results<Ok<TaskItem>, ProblemHttpResult> Get(int id, TaskRepository repo) =>
        repo.Find(id) is { } task ? TypedResults.Ok(task) : Problems.NotFound("task", id);

    static Results<Ok<TaskItem>, ProblemHttpResult> Replace(int id, TaskInput input, TaskRepository repo) =>
        repo.Replace(id, input) is { } task ? TypedResults.Ok(task) : Problems.NotFound("task", id);

    static Results<NoContent, ProblemHttpResult> Delete(int id, TaskRepository repo) =>
        repo.Remove(id) ? TypedResults.NoContent() : Problems.NotFound("task", id);

    public static async Task<int> RunAsync(string[] args)
    {
        var builder = ExampleHost.CreateBuilder(args);
        builder.Services.ConfigureHttpJsonOptions(o => Json.Configure(o.SerializerOptions));
        builder.Services.AddProblemDetails();
        builder.Services.AddSingleton<TimeProvider, StepClock>();
        builder.Services.AddSingleton<TaskRepository>();

        var app = builder.Build();
        app.UseStatusCodePages();

        var tasks = app.MapGroup("/tasks");
        tasks.MapPost("/", Create);
        tasks.MapGet("/{id:int}", Get);
        tasks.MapPut("/{id:int}", Replace);
        tasks.MapDelete("/{id:int}", Delete);

        await app.RunAsync();
        return 0;
    }
}
