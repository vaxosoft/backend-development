using System.Globalization;
using System.Text.Json;

namespace P6.JobWorkerPool;

/// <summary>Thrown by a handler to signal an ordinary job failure (which may be retried).</summary>
public sealed class JobFailedException(string message) : Exception(message);

/// <summary>Payload validation and the actual work of each job type.</summary>
public static class JobHandlers
{
    public const int MaxSleepMs = 5000;

    public static Dictionary<string, string[]> Validate(JobRequest request)
    {
        var errors = new Dictionary<string, string[]>();
        var payload = request.Payload ?? default;

        switch (request.Type)
        {
            case "reverse":
                if (payload.ValueKind != JsonValueKind.Object
                    || !payload.TryGetProperty("text", out var text)
                    || text.ValueKind != JsonValueKind.String)
                {
                    errors["payload.text"] = ["must be a string"];
                }

                break;
            case "sleep":
                if (payload.ValueKind != JsonValueKind.Object
                    || !payload.TryGetProperty("ms", out var ms)
                    || ms.ValueKind != JsonValueKind.Number
                    || !ms.TryGetInt32(out var value)
                    || value is < 0 or > MaxSleepMs)
                {
                    errors["payload.ms"] = [$"must be an integer between 0 and {MaxSleepMs}"];
                }

                break;
            case "fail":
                break;
            default:
                errors["type"] = ["must be one of: reverse, sleep, fail"];
                break;
        }

        return errors;
    }

    /// <summary>Runs one attempt. Must honour <paramref name="ct"/> so cancellation and timeouts take effect.</summary>
    public static async Task<object?> ExecuteAsync(Job job, TimeProvider time, CancellationToken ct)
    {
        switch (job.Type)
        {
            case "reverse":
                return Reverse(job.Payload.GetProperty("text").GetString()!);
            case "sleep":
                var ms = job.Payload.GetProperty("ms").GetInt32();
                await Task.Delay(TimeSpan.FromMilliseconds(ms), time, ct);
                return new { slept_ms = ms };
            default:
                throw new JobFailedException("job failed on purpose");
        }
    }

    // Reverse user-perceived characters, not UTF-16 chars: reversing a surrogate pair would corrupt emoji.
    private static string Reverse(string text)
    {
        var elements = new List<string>();
        var e = StringInfo.GetTextElementEnumerator(text);
        while (e.MoveNext())
        {
            elements.Add((string)e.Current);
        }

        elements.Reverse();
        return string.Concat(elements);
    }
}
