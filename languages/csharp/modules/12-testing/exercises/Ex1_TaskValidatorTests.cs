using M12.TaskApi;

namespace M12.Exercises;

public class Ex1_TaskValidatorTests
{
    private static readonly DateOnly Today = new(2030, 1, 15);

    [Theory]
    [InlineData("Write docs", true)]
    [InlineData("a", true)]
    [InlineData("  padded  ", true)]
    [InlineData("", false)]
    [InlineData("   ", false)]
    [InlineData(null, false)]
    public void Title_MustHaveContent(string? title, bool valid) =>
        Assert.Equal(valid, !Ex1_TaskValidator.Validate(new NewTask(title, null), Today).ContainsKey("title"));

    [Theory]
    [InlineData(200, true)]
    [InlineData(201, false)]
    public void Title_LengthLimitAppliesToTheTrimmedValue(int length, bool valid)
    {
        var title = new string('x', length);

        Assert.Equal(valid, Ex1_TaskValidator.Validate(new NewTask(title, null), Today).Count == 0);
        Assert.True(Ex1_TaskValidator.Validate(new NewTask($"  {new string('x', 200)}  ", null), Today).Count == 0);
    }

    public static TheoryData<DateOnly?, bool> DueDates => new()
    {
        { null, true },
        { new DateOnly(2030, 1, 15), true },
        { new DateOnly(2030, 1, 16), true },
        { new DateOnly(2030, 1, 14), false },
        { new DateOnly(1999, 12, 31), false },
    };

    [Theory]
    [MemberData(nameof(DueDates))]
    public void DueDate_MayNotBeInThePast(DateOnly? due, bool valid) =>
        Assert.Equal(valid, !Ex1_TaskValidator.Validate(new NewTask("ok", due), Today).ContainsKey("due_date"));

    [Fact]
    public void Messages_AreStable()
    {
        var errors = Ex1_TaskValidator.Validate(new NewTask("", new DateOnly(2030, 1, 1)), Today);

        Assert.Equal("must be 1-200 characters", errors["title"]);
        Assert.Equal("must not be in the past", errors["due_date"]);
        Assert.Equal(2, errors.Count);
    }

    [Fact]
    public void ValidTask_HasNoErrors() =>
        Assert.Empty(Ex1_TaskValidator.Validate(new NewTask("Ship it", new DateOnly(2030, 2, 1)), Today));
}
