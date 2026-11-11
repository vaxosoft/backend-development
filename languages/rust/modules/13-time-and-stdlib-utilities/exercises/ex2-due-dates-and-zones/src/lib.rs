//! Exercise 2: due dates and time zones. Store UTC instants, compare calendar dates, and handle DST.

use chrono::{DateTime, NaiveDate, Utc};
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
pub fn parse_due_date(_text: &str) -> Result<NaiveDate, TimeError> {
    todo!("chrono's parser is lenient about padding: check the shape yourself")
}

pub fn parse_zone(_name: &str) -> Result<Tz, TimeError> {
    todo!("Tz implements FromStr")
}

/// Whole calendar days from `today` until `due`; negative once the date has passed.
pub fn days_until(_due: NaiveDate, _today: NaiveDate) -> i64 {
    todo!()
}

/// Overdue means strictly before today: a task due today is not overdue yet.
pub fn is_overdue(_due: NaiveDate, _today: NaiveDate) -> bool {
    todo!()
}

/// The calendar date it currently is in `tz` (not in UTC).
pub fn local_today(_now: DateTime<Utc>, _tz: Tz) -> NaiveDate {
    todo!("convert the instant to the zone, then take the date")
}

/// First instant of `date` in `tz`. If local midnight does not exist (a DST gap) use the first
/// local time that does.
pub fn start_of_day_utc(_date: NaiveDate, _tz: Tz) -> DateTime<Utc> {
    todo!("handle all three LocalResult variants: Single, Ambiguous, None")
}

/// Last whole second (23:59:59) of `date` in `tz`. If that local time occurs twice (DST fall back),
/// use the later one.
pub fn end_of_day_utc(_date: NaiveDate, _tz: Tz) -> DateTime<Utc> {
    todo!("handle all three LocalResult variants: Single, Ambiguous, None")
}
