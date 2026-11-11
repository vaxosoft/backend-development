//! chrono basics: instants, calendar dates, durations and formatting. No clock reads, so the output is stable.

use chrono::{DateTime, Datelike, Duration, NaiveDate, SecondsFormat, TimeZone, Utc, Weekday};

fn main() {
    // An instant (DateTime<Utc>) vs a calendar date (NaiveDate): a due date has no time or zone.
    let created_at: DateTime<Utc> = Utc.with_ymd_and_hms(2030, 1, 1, 9, 0, 0).unwrap();
    let due: NaiveDate = NaiveDate::from_ymd_opt(2030, 1, 31).unwrap();
    println!(
        "created_at: {}",
        created_at.to_rfc3339_opts(SecondsFormat::Secs, true)
    );
    println!("due_date:   {due}");

    // Parsing: RFC 3339 with an offset becomes an instant; date-only text becomes a NaiveDate.
    let parsed = DateTime::parse_from_rfc3339("2030-01-01T10:30:00+01:00").unwrap();
    println!(
        "offset input in UTC: {}",
        parsed
            .with_timezone(&Utc)
            .to_rfc3339_opts(SecondsFormat::Secs, true)
    );
    println!(
        "bad date: {}",
        NaiveDate::parse_from_str("2030-02-30", "%Y-%m-%d").is_err()
    );

    // Arithmetic with chrono::Duration (alias of TimeDelta): signed and calendar-agnostic.
    let reminder = created_at + Duration::hours(36);
    println!(
        "created + 36h: {}",
        reminder.to_rfc3339_opts(SecondsFormat::Secs, true)
    );
    let days_left = (due - created_at.date_naive()).num_days();
    println!("days until due: {days_left}");
    let waited = reminder - created_at;
    println!(
        "waited: {} minutes ({:?})",
        waited.num_minutes(),
        waited.to_std().unwrap()
    );

    // Calendar arithmetic is not duration arithmetic: months have different lengths.
    let jan31 = NaiveDate::from_ymd_opt(2030, 1, 31).unwrap();
    println!(
        "Jan 31 + 1 month: {:?}",
        jan31.checked_add_months(chrono::Months::new(1))
    );
    println!("Jan 31 + 30 days: {}", jan31 + Duration::days(30));
    println!(
        "weekday of due: {:?}, is weekend: {}",
        due.weekday(),
        matches!(due.weekday(), Weekday::Sat | Weekday::Sun)
    );

    // Formatting
    println!("custom: {}", created_at.format("%a %d %b %Y, %H:%M"));

    // Three Duration types: std (unsigned, for timeouts), chrono (signed, for calendars), tokio re-exports std.
    let timeout: std::time::Duration = std::time::Duration::from_millis(1500);
    let signed: Duration = Duration::from_std(timeout).unwrap();
    println!(
        "std {:?} -> chrono {}ms",
        timeout,
        signed.num_milliseconds()
    );
    println!(
        "a negative chrono duration cannot become a std one: {}",
        (-signed).to_std().is_err()
    );
}
