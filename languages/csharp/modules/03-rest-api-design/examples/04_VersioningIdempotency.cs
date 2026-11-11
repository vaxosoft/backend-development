using System.Collections.Concurrent;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using Microsoft.AspNetCore.Http.HttpResults;
using Microsoft.AspNetCore.Mvc;

namespace M03.Examples;

/// <summary>Versioning by route group (with deprecation headers) and idempotent POST via Idempotency-Key.</summary>
static class VersioningIdempotency
{
    /// <summary>The old, flatter shape that v1 clients still rely on.</summary>
    sealed record TaskV1(int Id, string Title, bool Done);

    sealed class IdempotencyStore
    {
        public sealed record Entry(string Fingerprint, TaskItem Task);

        readonly ConcurrentDictionary<string, Lazy<Entry>> _entries = [];

        /// <summary>Runs <paramref name="create"/> at most once per key, even under concurrent retries.</summary>
        public (Entry Entry, bool Replayed) GetOrCreate(string key, string fingerprint, Func<TaskItem> create)
        {
            var mine = new Lazy<Entry>(() => new Entry(fingerprint, create()));
            var winner = _entries.GetOrAdd(key, mine);
            return (winner.Value, !ReferenceEquals(winner, mine));
        }
    }

    static string Fingerprint(TaskInput input) =>
        Convert.ToHexString(SHA256.HashData(Encoding.UTF8.GetBytes(JsonSerializer.Serialize(input))));

    static Results<Created<TaskItem>, ProblemHttpResult> CreateV2(
        TaskInput input,
        TaskRepository repo,
        IdempotencyStore store,
        HttpContext context,
        [FromHeader(Name = "Idempotency-Key")] string? key)
    {
        if (string.IsNullOrWhiteSpace(key))
        {
            var created = repo.Add(input);
            return TypedResults.Created($"/v2/tasks/{created.Id}", created);
        }

        var (entry, replayed) = store.GetOrCreate(key, Fingerprint(input), () => repo.Add(input));
        if (entry.Fingerprint != Fingerprint(input))
        {
            return Problems.Invalid("idempotency-key", "was already used with a different request body");
        }

        if (replayed)
        {
            context.Response.Headers["Idempotent-Replayed"] = "true";
        }

        return TypedResults.Created($"/v2/tasks/{entry.Task.Id}", entry.Task);
    }

    public static async Task<int> RunAsync(string[] args)
    {
        var builder = ExampleHost.CreateBuilder(args);
        builder.Services.ConfigureHttpJsonOptions(o => Json.Configure(o.SerializerOptions));
        builder.Services.AddValidatedProblemDetails();
        builder.Services.AddSingleton<TimeProvider, StepClock>();
        builder.Services.AddSingleton<TaskRepository>();
        builder.Services.AddSingleton<IdempotencyStore>();

        var app = builder.Build();
        app.UseStatusCodePages();

        var v1 = app.MapGroup("/v1/tasks");
        v1.AddEndpointFilter(async (context, next) =>
        {
            var headers = context.HttpContext.Response.Headers;
            headers["Deprecation"] = "true";
            headers["Sunset"] = "Sat, 31 Dec 2033 23:59:59 GMT";
            headers.Link = "</v2/tasks>; rel=\"successor-version\"";
            return await next(context);
        });
        v1.MapGet("/", (TaskRepository repo) => repo.All().Select(t => new TaskV1(t.Id, t.Title, t.Status == TaskState.Done)));

        var v2 = app.MapGroup("/v2/tasks");
        v2.MapPost("/", CreateV2);
        v2.MapGet("/", (TaskRepository repo) => repo.All());

        await app.RunAsync();
        return 0;
    }
}
