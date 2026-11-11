using System.Text.Json;

namespace M04.Exercises;

/// <summary>Result of applying a PATCH body: the new task, or a malformed-body flag, or per-field errors.</summary>
public sealed record PatchOutcome(TaskDto? Task, bool Malformed, IReadOnlyDictionary<string, string> Errors)
{
    public bool Ok => Task is not null;
}

public static class Ex3_Patch
{
    /// <summary>
    /// Apply a JSON merge-style PATCH to <paramref name="current"/>.
    /// Only title, status, due_date and tags may appear. Absent = unchanged; due_date null clears it.
    /// Not JSON / not an object => Malformed (400). Otherwise collect every field error (422) and return no task:
    /// title trimmed 1-200 chars (null not allowed); status todo|in_progress|done; due_date YYYY-MM-DD or null;
    /// tags an array of at most 10 unique lower-case strings of 1-30 chars (null not allowed);
    /// any other member (id, owner_id, created_at, ...) is an error "unknown or read-only field".
    /// </summary>
    public static PatchOutcome Apply(TaskDto current, string json) => throw new NotImplementedException();
}
