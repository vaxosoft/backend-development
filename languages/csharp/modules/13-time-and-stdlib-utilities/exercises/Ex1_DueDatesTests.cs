namespace M13.Exercises;

public class Ex1_DueDatesTests
{
    private static readonly TimeZoneInfo Plus2 = TimeZoneInfo.CreateCustomTimeZone("plus2", TimeSpan.FromHours(2), "+2", "+2");
    private static readonly TimeZoneInfo Minus5 = TimeZoneInfo.CreateCustomTimeZone("minus5", TimeSpan.FromHours(-5), "-5", "-5");

    // 23:30 UTC on Jan 31: already Feb 1 in UTC+2, still Jan 31 in UTC-5.
    private static readonly DateTimeOffset Now = new(2030, 1, 31, 23, 30, 0, TimeSpan.Zero);

    [Fact]
    public void LocalToday_DependsOnTheZone()
    {
        Assert.Equal(new DateOnly(2030, 2, 1), Ex1_DueDates.LocalToday(Now, Plus2));
        Assert.Equal(new DateOnly(2030, 1, 31), Ex1_DueDates.LocalToday(Now, Minus5));
        Assert.Equal(new DateOnly(2030, 1, 31), Ex1_DueDates.LocalToday(Now, TimeZoneInfo.Utc));
    }

    [Fact]
    public void LocalToday_IgnoresTheOffsetCarriedByTheInstant()
    {
        var sameInstantOtherOffset = Now.ToOffset(TimeSpan.FromHours(9));

        Assert.Equal(new DateOnly(2030, 1, 31), Ex1_DueDates.LocalToday(sameInstantOtherOffset, Minus5));
    }

    [Theory]
    [InlineData("2030-01-31", false)]
    [InlineData("2030-01-30", true)]
    [InlineData("2030-02-01", false)]
    public void IsOverdue_UsesTheLocalDate_InUtcMinus5(string due, bool expected) =>
        Assert.Equal(expected, Ex1_DueDates.IsOverdue(DateOnly.Parse(due), Now, Minus5));

    [Fact]
    public void IsOverdue_TheSameDueDateIsOverdueEarlierInTheEast()
    {
        var due = new DateOnly(2030, 1, 31);

        Assert.False(Ex1_DueDates.IsOverdue(due, Now, Minus5));
        Assert.True(Ex1_DueDates.IsOverdue(due, Now, Plus2));
        Assert.False(Ex1_DueDates.IsOverdue(null, Now, Plus2));
    }

    [Fact]
    public void DaysUntilDue_CountsCalendarDays()
    {
        Assert.Equal(0, Ex1_DueDates.DaysUntilDue(new DateOnly(2030, 1, 31), Now, Minus5));
        Assert.Equal(10, Ex1_DueDates.DaysUntilDue(new DateOnly(2030, 2, 10), Now, Minus5));
        Assert.Equal(-1, Ex1_DueDates.DaysUntilDue(new DateOnly(2030, 1, 31), Now, Plus2));
        Assert.Null(Ex1_DueDates.DaysUntilDue(null, Now, Plus2));
    }

    [Fact]
    public void EndOfDayUtc_ShiftsByTheZoneOffset()
    {
        var end = Ex1_DueDates.EndOfDayUtc(new DateOnly(2030, 1, 31), Plus2);

        Assert.Equal(new DateTimeOffset(2030, 1, 31, 21, 59, 59, TimeSpan.Zero), end);
        Assert.Equal(TimeSpan.Zero, end.Offset);
    }

    [Fact]
    public void EndOfDayUtc_FollowsDaylightSavingTime()
    {
        var berlin = TimeZoneInfo.FindSystemTimeZoneById("Europe/Berlin");

        Assert.Equal(new DateTimeOffset(2030, 1, 31, 22, 59, 59, TimeSpan.Zero), Ex1_DueDates.EndOfDayUtc(new DateOnly(2030, 1, 31), berlin));
        Assert.Equal(new DateTimeOffset(2030, 7, 1, 21, 59, 59, TimeSpan.Zero), Ex1_DueDates.EndOfDayUtc(new DateOnly(2030, 7, 1), berlin));
    }
}
