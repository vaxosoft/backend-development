using System.Collections.Concurrent;

namespace M03.Exercises;

/// <summary>A response worth remembering so that a retry can be answered without redoing the work.</summary>
public sealed record StoredResponse(int StatusCode, string Body, string? Location);

public enum IdempotencyOutcome
{
    /// <summary>The action ran now.</summary>
    Executed,

    /// <summary>The key was seen before with the same request; the stored response is returned.</summary>
    Replayed,

    /// <summary>The key was seen before with a different request; nothing ran.</summary>
    Mismatch,
}

public sealed record IdempotencyResult(IdempotencyOutcome Outcome, StoredResponse? Response);

/// <summary>Makes a non-idempotent operation (POST) safe to retry when the client sends an Idempotency-Key.</summary>
public sealed class IdempotencyStore
{
    sealed record Entry(string Fingerprint, Task<StoredResponse> Response);

    readonly ConcurrentDictionary<string, Entry> _entries = [];

    /// <summary>
    /// <list type="bullet">
    /// <item>first call for a key: run <paramref name="action"/>, remember its response, outcome Executed</item>
    /// <item>same key and same fingerprint: do not run the action, return the stored response, outcome Replayed</item>
    /// <item>same key, different fingerprint: do not run the action, outcome Mismatch, no response</item>
    /// <item>concurrent calls with one key run the action exactly once; the others wait for its response</item>
    /// <item>if the action throws, nothing is remembered (the key can be retried) and the exception propagates</item>
    /// </list>
    /// </summary>
    public async Task<IdempotencyResult> ExecuteAsync(string key, string fingerprint, Func<Task<StoredResponse>> action)
    {
        var mine = new TaskCompletionSource<StoredResponse>(TaskCreationOptions.RunContinuationsAsynchronously);
        var entry = _entries.GetOrAdd(key, new Entry(fingerprint, mine.Task));

        if (!ReferenceEquals(entry.Response, mine.Task))
        {
            return entry.Fingerprint != fingerprint
                ? new IdempotencyResult(IdempotencyOutcome.Mismatch, null)
                : new IdempotencyResult(IdempotencyOutcome.Replayed, await entry.Response);
        }

        try
        {
            var response = await action();
            mine.SetResult(response);
            return new IdempotencyResult(IdempotencyOutcome.Executed, response);
        }
        catch (Exception ex)
        {
            _entries.TryRemove(key, out _); // a failed attempt must not poison the key
            mine.SetException(ex);
            throw;
        }
    }
}

public static class PaymentEndpoints
{
    /// <summary>
    /// POST /tasks creates a task and answers 201 + Location. With an Idempotency-Key header a retry gets the
    /// original response plus "Idempotent-Replayed: true"; reusing a key with another body is a 422.
    /// </summary>
    public static void MapCreateTask(IEndpointRouteBuilder routes, IdempotencyStore store, Action onCreated)
    {
        var lastId = 0;
        routes.MapPost("/tasks", async (HttpContext context) =>
        {
            using var reader = new StreamReader(context.Request.Body);
            var body = await reader.ReadToEndAsync();

            Task<StoredResponse> Create()
            {
                onCreated();
                var id = Interlocked.Increment(ref lastId);
                return Task.FromResult(new StoredResponse(StatusCodes.Status201Created, body, $"/tasks/{id}"));
            }

            if (context.Request.Headers["Idempotency-Key"].FirstOrDefault() is not { Length: > 0 } key)
            {
                return new StoredResult(await Create(), replayed: false);
            }

            var result = await store.ExecuteAsync(key, body, Create);
            return result.Outcome == IdempotencyOutcome.Mismatch
                ? Results.Problem(statusCode: StatusCodes.Status422UnprocessableEntity, title: "Idempotency-Key reuse")
                : new StoredResult(result.Response!, result.Outcome == IdempotencyOutcome.Replayed);
        });
    }

    sealed class StoredResult(StoredResponse response, bool replayed) : IResult
    {
        public Task ExecuteAsync(HttpContext context)
        {
            context.Response.StatusCode = response.StatusCode;
            context.Response.ContentType = "application/json";
            if (response.Location is not null)
            {
                context.Response.Headers.Location = response.Location;
            }

            if (replayed)
            {
                context.Response.Headers["Idempotent-Replayed"] = "true";
            }

            return context.Response.WriteAsync(response.Body);
        }
    }
}
