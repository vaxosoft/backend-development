namespace M13.Exercises;

/// <summary>Calendar-day logic for due dates, always relative to the user's time zone.</summary>
public static class Ex1_DueDates
{
    /// <summary>The calendar date it currently is in <paramref name="zone"/>.</summary>
    public static DateOnly LocalToday(DateTimeOffset now, TimeZoneInfo zone) =>
        DateOnly.FromDateTime(TimeZoneInfo.ConvertTime(now, zone).DateTime);

    /// <summary>True when a due date exists and is before the local date. Due today is not overdue.</summary>
    public static bool IsOverdue(DateOnly? due, DateTimeOffset now, TimeZoneInfo zone) =>
        due < LocalToday(now, zone);

    /// <summary>Whole days from local today to <paramref name="due"/> (negative when overdue); null without a due date.</summary>
    public static int? DaysUntilDue(DateOnly? due, DateTimeOffset now, TimeZoneInfo zone) =>
        due?.DayNumber - LocalToday(now, zone).DayNumber;

    /// <summary>The instant, as UTC, of 23:59:59 on <paramref name="due"/> in <paramref name="zone"/> (DST-aware).</summary>
    public static DateTimeOffset EndOfDayUtc(DateOnly due, TimeZoneInfo zone)
    {
        var local = due.ToDateTime(new TimeOnly(23, 59, 59));
        return new DateTimeOffset(local, zone.GetUtcOffset(local)).ToUniversalTime();
    }
}
