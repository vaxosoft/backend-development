using System.Text.Json;

namespace M04.Exercises;

public class Ex1_WireTests
{
    private static readonly TaskDto Sample = new(1, "Write docs", Status.InProgress, new DateOnly(2030, 1, 31), ["docs", "p1"], null);

    [Fact]
    public void Serializes_with_snake_case_names_and_string_enums()
    {
        Assert.Equal(
            """{"id":1,"title":"Write docs","status":"in_progress","due_date":"2030-01-31","tags":["docs","p1"],"owner_id":null}""",
            Ex1_Wire.ToJson(Sample));
    }

    [Fact]
    public void Reads_case_insensitively_and_ignores_unknown_members()
    {
        var task = Ex1_Wire.FromJson("""{"ID":2,"Title":"x","STATUS":"done","due_date":null,"tags":[],"owner_id":7,"extra":true}""");

        Assert.Equal(2, task.Id);
        Assert.Equal(Status.Done, task.Status);
        Assert.Null(task.DueDate);
        Assert.Equal(7, task.OwnerId);
    }

    [Fact]
    public void Round_trips()
    {
        var back = Ex1_Wire.FromJson(Ex1_Wire.ToJson(Sample));

        Assert.Equal(Sample.Title, back.Title);
        Assert.Equal(Sample.Status, back.Status);
        Assert.Equal(Sample.DueDate, back.DueDate);
        Assert.Equal(Sample.Tags, back.Tags);
    }

    [Theory]
    [InlineData("""{"id":1,"title":"x","status":"finished","tags":[]}""")]
    [InlineData("""{"id":1,"title":"x","status":1,"tags":[]}""")]
    public void Rejects_unknown_enum_names_and_numbers(string json)
    {
        Assert.Throws<JsonException>(() => Ex1_Wire.FromJson(json));
    }
}
