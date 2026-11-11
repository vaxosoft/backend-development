namespace M04.Examples;

public enum Status
{
    Todo,
    InProgress,
    Done,
}

public sealed record TaskItem(
    int Id,
    string Title,
    Status Status,
    DateOnly? DueDate,
    string[] Tags,
    int? OwnerId,
    DateTimeOffset CreatedAt);
