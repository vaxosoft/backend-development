namespace M04.Exercises;

public class Ex3_PatchTests
{
    private static readonly TaskDto Current = new(5, "Original", Status.Todo, new DateOnly(2030, 1, 31), ["docs"], 9);

    [Fact]
    public void Absent_members_stay_unchanged()
    {
        var outcome = Ex3_Patch.Apply(Current, """{"status":"done"}""");

        Assert.True(outcome.Ok);
        Assert.Equal(Current with { Status = Status.Done }, outcome.Task, new TaskComparer());
    }

    [Fact]
    public void Null_due_date_clears_it_but_absent_keeps_it()
    {
        Assert.Null(Ex3_Patch.Apply(Current, """{"due_date":null}""").Task!.DueDate);
        Assert.Equal(new DateOnly(2030, 1, 31), Ex3_Patch.Apply(Current, """{"title":"x"}""").Task!.DueDate);
    }

    [Fact]
    public void Title_is_trimmed_and_tags_replaced()
    {
        var task = Ex3_Patch.Apply(Current, """{"title":"  New  ","tags":["a","b"],"due_date":"2031-02-03"}""").Task!;

        Assert.Equal("New", task.Title);
        Assert.Equal(["a", "b"], task.Tags);
        Assert.Equal(new DateOnly(2031, 2, 3), task.DueDate);
        Assert.Equal(9, task.OwnerId);
    }

    [Theory]
    [InlineData("{")]
    [InlineData("[]")]
    [InlineData("\"text\"")]
    [InlineData("")]
    public void Malformed_bodies_are_flagged(string json)
    {
        var outcome = Ex3_Patch.Apply(Current, json);

        Assert.True(outcome.Malformed);
        Assert.False(outcome.Ok);
    }

    [Theory]
    [InlineData("""{"title":null}""", "title")]
    [InlineData("""{"title":"   "}""", "title")]
    [InlineData("""{"title":7}""", "title")]
    [InlineData("""{"status":"finished"}""", "status")]
    [InlineData("""{"status":null}""", "status")]
    [InlineData("""{"due_date":"31/01/2030"}""", "due_date")]
    [InlineData("""{"due_date":20300131}""", "due_date")]
    [InlineData("""{"tags":null}""", "tags")]
    [InlineData("""{"tags":["A"]}""", "tags")]
    [InlineData("""{"tags":["a","a"]}""", "tags")]
    [InlineData("""{"tags":[1]}""", "tags")]
    [InlineData("""{"id":6}""", "id")]
    [InlineData("""{"owner_id":null}""", "owner_id")]
    public void Invalid_values_are_reported_per_field(string json, string field)
    {
        var outcome = Ex3_Patch.Apply(Current, json);

        Assert.False(outcome.Ok);
        Assert.False(outcome.Malformed);
        Assert.Contains(field, outcome.Errors.Keys);
    }

    [Fact]
    public void All_errors_are_collected_and_the_task_is_not_partially_applied()
    {
        var outcome = Ex3_Patch.Apply(Current, """{"title":"ok","status":"nope","tags":["X"]}""");

        Assert.Null(outcome.Task);
        Assert.Equal(["status", "tags"], outcome.Errors.Keys.Order());
    }

    [Fact]
    public void Eleven_tags_are_rejected()
    {
        var tags = string.Join(',', Enumerable.Range(0, 11).Select(i => $"\"t{i}\""));

        Assert.Contains("tags", Ex3_Patch.Apply(Current, $$"""{"tags":[{{tags}}]}""").Errors.Keys);
    }

    // Records compare arrays by reference, so compare the fields by value.
    private sealed class TaskComparer : IEqualityComparer<TaskDto?>
    {
        public bool Equals(TaskDto? x, TaskDto? y) =>
            x is not null && y is not null &&
            (x.Id, x.Title, x.Status, x.DueDate, x.OwnerId) == (y.Id, y.Title, y.Status, y.DueDate, y.OwnerId) &&
            x.Tags.SequenceEqual(y.Tags);

        public int GetHashCode(TaskDto? obj) => obj?.Id ?? 0;
    }
}
