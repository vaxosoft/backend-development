//! Exercise 2: due dates and time zones. Store UTC instants, compare calendar dates, and handle DST.

use chrono::{DateTime, Duration, LocalResult, NaiveDate, NaiveDateTime, TimeZone, Utc};
use chrono_tz::Tz;

#[derive(Debug, PartialEq, Eq, thiserror::Error)]
pub enum TimeError {
    #[error("due date must look like YYYY-MM-DD")]
    BadDate,
    #[error("unknown time zone {0:?}")]
    UnknownZone(String),
}

/// Strict `YYYY-MM-DD`: exactly four, two and two digits, and a date that exists.
/// `"2030-1-5"`, `" 2030-01-05"`, `"2030-02-30"` and `"2030-01-05T00:00"` are all `BadDate`.
pub fn parse_due_date(text: &str) -> Result<NaiveDate, TimeError> {
    let bytes = text.as_bytes();
    let shape_ok = bytes.len() == 10
        && bytes.iter().enumerate().all(|(i, b)| match i {
            4 | 7 => *b == b'-',
            _ => b.is_ascii_digit(),
        });
    if !shape_ok {
        return Err(TimeError::BadDate);
    }
    NaiveDate::parse_from_str(text, "%Y-%m-%d").map_err(|_| TimeError::BadDate)
}

pub fn parse_zone(name: &str) -> Result<Tz, TimeError> {
    name.parse::<Tz>()
        .map_err(|_| TimeError::UnknownZone(name.to_string()))
}

/// Whole calendar days from `today` until `due`; negative once the date has passed.
pub fn days_until(due: NaiveDate, today: NaiveDate) -> i64 {
    (due - today).num_days()
}

/// Overdue means strictly before today: a task due today is not overdue yet.
pub fn is_overdue(due: NaiveDate, today: NaiveDate) -> bool {
    due < today
}

/// The calendar date it currently is in `tz` (not in UTC).
pub fn local_today(now: DateTime<Utc>, tz: Tz) -> NaiveDate {
    now.with_timezone(&tz).date_naive()
}

fn to_utc(local: NaiveDateTime, tz: Tz, prefer_later: bool) -> Option<DateTime<Utc>> {
    match tz.from_local_datetime(&local) {
        LocalResult::Single(t) => Some(t.with_timezone(&Utc)),
        LocalResult::Ambiguous(first, second) => {
            Some(if prefer_later { second } else { first }.with_timezone(&Utc))
        }
        LocalResult::None => None,
    }
}

/// First instant of `date` in `tz`. If local midnight does not exist (a DST gap) use the first
/// local time that does.
pub fn start_of_day_utc(date: NaiveDate, tz: Tz) -> DateTime<Utc> {
    let midnight = date.and_hms_opt(0, 0, 0).expect("00:00:00 is a valid time");
    (0..=48)
        .find_map(|step| to_utc(midnight + Duration::minutes(30 * step), tz, false))
        .expect("every zone has valid local times within a day")
}

/// Last whole second (23:59:59) of `date` in `tz`. If that local time occurs twice (DST fall back),
/// use the later one.
pub fn end_of_day_utc(date: NaiveDate, tz: Tz) -> DateTime<Utc> {
    let last = date
        .and_hms_opt(23, 59, 59)
        .expect("23:59:59 is a valid time");
    (0..=48)
        .find_map(|step| to_utc(last - Duration::minutes(30 * step), tz, true))
        .expect("every zone has valid local times within a day")
}
