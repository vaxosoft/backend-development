namespace M04.Exercises;

public enum Status
{
    Todo,
    InProgress,
    Done,
}

/// <summary>A task as the API sees it (read-only fields such as created_at are omitted).</summary>
public sealed record TaskDto(int Id, string Title, Status Status, DateOnly? DueDate, string[] Tags, int? OwnerId);
