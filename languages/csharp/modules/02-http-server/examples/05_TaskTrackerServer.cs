using System.Text.Json;
using System.Text.Json.Serialization;

namespace M02.Examples;

/// <summary>A small Task Tracker server: DI lifetimes, route groups, endpoint filters, exception handling.</summary>
static class TaskTrackerServer
{
    enum TaskState
    {
        Todo,
        InProgress,
        Done,
    }

    record TaskItem(int Id, string Title, TaskState Status, DateOnly? DueDate, string[] Tags, int? OwnerId, DateTime CreatedAt);

    record CreateTask(string Title, DateOnly? DueDate, string[]? Tags);

    interface ITaskStore
    {
        TaskItem Add(CreateTask request);

        TaskItem? Find(int id);

        IReadOnlyList<TaskItem> All();
    }

    sealed class InMemoryTaskStore(TimeProvider clock) : ITaskStore
    {
        readonly Lock _gate = new();
        readonly List<TaskItem> _items = [];

        public TaskItem Add(CreateTask request)
        {
            lock (_gate)
            {
                var item = new TaskItem(_items.Count + 1, request.Title.Trim(), TaskState.Todo, request.DueDate,
                    request.Tags ?? [], null, clock.GetUtcNow().UtcDateTime);
                _items.Add(item);
                return item;
            }
        }

        public TaskItem? Find(int id)
        {
            lock (_gate)
            {
                return _items.FirstOrDefault(t => t.Id == id);
            }
        }

        public IReadOnlyList<TaskItem> All()
        {
            lock (_gate)
            {
                return [.. _items];
            }
        }
    }

    sealed class FixedClock : TimeProvider
    {
        public override DateTimeOffset GetUtcNow() => new(2030, 1, 1, 9, 0, 0, TimeSpan.Zero);
    }

    // Three lifetimes, each numbering its instances so the difference is visible.
    sealed class SingletonDep
    {
        public int Id { get; } = 1;
    }

    sealed class ScopedDep
    {
        static int s_next;

        public int Id { get; } = Interlocked.Increment(ref s_next);
    }

    sealed class TransientDep
    {
        static int s_next;

        public int Id { get; } = Interlocked.Increment(ref s_next);
    }

    static async ValueTask<object?> PositiveId(EndpointFilterInvocationContext context, EndpointFilterDelegate next)
    {
        if (context.Arguments.OfType<int>().FirstOrDefault() <= 0)
        {
            return Results.BadRequest("id must be positive");
        }

        return await next(context);
    }

    public static async Task<int> RunAsync(string[] args)
    {
        var builder = ExampleHost.CreateBuilder(args);
        builder.Services.ConfigureHttpJsonOptions(o =>
        {
            o.SerializerOptions.PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower;
            o.SerializerOptions.Converters.Add(new JsonStringEnumConverter(JsonNamingPolicy.SnakeCaseLower));
        });
        builder.Services.AddProblemDetails();
        builder.Services.AddSingleton<TimeProvider, FixedClock>();
        builder.Services.AddSingleton<ITaskStore, InMemoryTaskStore>();
        builder.Services.AddSingleton<SingletonDep>();
        builder.Services.AddScoped<ScopedDep>();
        builder.Services.AddTransient<TransientDep>();

        var app = builder.Build();
        app.UseExceptionHandler();

        app.MapGet("/healthz", () => new { status = "ok" });
        app.MapGet("/boom", string () => throw new InvalidOperationException("secret detail"));

        app.MapGet("/lifetimes", (SingletonDep singleton, ScopedDep scoped, HttpContext context) => new
        {
            singleton = singleton.Id,
            scoped = scoped.Id,
            scoped_again = context.RequestServices.GetRequiredService<ScopedDep>().Id,
            transient = new[]
            {
                context.RequestServices.GetRequiredService<TransientDep>().Id,
                context.RequestServices.GetRequiredService<TransientDep>().Id,
            },
        });

        var tasks = app.MapGroup("/tasks");
        tasks.AddEndpointFilter(async (context, next) =>
        {
            context.HttpContext.Response.Headers["X-Group"] = "tasks";
            return await next(context);
        });

        tasks.MapGet("/", (ITaskStore store) => store.All());
        tasks.MapPost("/", (CreateTask request, ITaskStore store) =>
        {
            if (string.IsNullOrWhiteSpace(request.Title))
            {
                return Results.BadRequest("title is required");
            }

            var task = store.Add(request);
            return TypedResults.Created($"/tasks/{task.Id}", task);
        });
        tasks.MapGet("/{id:int}", (int id, ITaskStore store) =>
                store.Find(id) is { } task ? Results.Ok(task) : Results.NotFound())
            .AddEndpointFilter(PositiveId);

        await app.RunAsync();
        return 0;
    }
}
