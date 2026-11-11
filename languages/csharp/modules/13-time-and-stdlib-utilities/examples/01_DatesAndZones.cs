using System.Globalization;

namespace M13.Examples;

public static class DatesAndZones
{
    public static Task<int> RunAsync(string[] args)
    {
        var berlin = TimeZoneInfo.FindSystemTimeZoneById("Europe/Berlin");

        // Store and exchange instants in UTC; convert to a zone only to display or to decide "which day".
        var instant = new DateTimeOffset(2030, 1, 31, 23, 30, 0, TimeSpan.Zero);
        var local = TimeZoneInfo.ConvertTime(instant, berlin);
        Console.WriteLine($"UTC    {instant:O}");
        Console.WriteLine($"Berlin {local:O}");
        Console.WriteLine($"same instant: {instant == local}, same calendar day: {instant.Date == local.Date}");

        var winter = TimeZoneInfo.ConvertTime(new DateTimeOffset(2030, 3, 31, 0, 30, 0, TimeSpan.Zero), berlin);
        var summer = TimeZoneInfo.ConvertTime(new DateTimeOffset(2030, 3, 31, 1, 30, 0, TimeSpan.Zero), berlin);
        Console.WriteLine($"offset before DST change: {winter.Offset}, one hour later: {summer.Offset}");

        // Local wall-clock times that do not exist or happen twice around DST changes.
        Console.WriteLine($"2030-03-31 02:30 invalid: {berlin.IsInvalidTime(new DateTime(2030, 3, 31, 2, 30, 0))}");
        Console.WriteLine($"2030-10-27 02:30 ambiguous: {berlin.IsAmbiguousTime(new DateTime(2030, 10, 27, 2, 30, 0))}");

        var round = DateTimeOffset.Parse(instant.ToString("O"), CultureInfo.InvariantCulture);
        Console.WriteLine($"ISO 8601 round trip: {round == instant}");

        var due = DateOnly.Parse("2030-01-31", CultureInfo.InvariantCulture);
        var endOfDay = new DateTimeOffset(due.ToDateTime(new TimeOnly(23, 59, 59)), berlin.GetUtcOffset(due.ToDateTime(TimeOnly.MinValue)));
        Console.WriteLine($"due date end of day in Berlin as UTC: {endOfDay.UtcDateTime:O}");

        var span = new DateTimeOffset(2030, 2, 3, 12, 0, 0, TimeSpan.Zero) - instant;
        Console.WriteLine($"elapsed: {span} = {span.TotalHours:0.#} hours");

        var unspecified = new DateTime(2030, 1, 31, 12, 0, 0);
        Console.WriteLine($"DateTime.Kind of a plain DateTime: {unspecified.Kind}");
        return Task.FromResult(0);
    }
}
