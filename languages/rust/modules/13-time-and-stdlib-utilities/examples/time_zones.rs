//! Time zones with chrono-tz: store UTC, convert for display, and handle DST edge cases.

use chrono::{DateTime, LocalResult, NaiveDate, NaiveDateTime, SecondsFormat, TimeZone, Utc};
use chrono_tz::Tz;

fn show(dt: DateTime<Tz>) -> String {
    dt.to_rfc3339_opts(SecondsFormat::Secs, false)
}

fn main() {
    let instant = Utc.with_ymd_and_hms(2030, 7, 1, 12, 0, 0).unwrap();

    // Zone names come from users/config: parse them, never hard-code offsets.
    for name in [
        "Europe/Berlin",
        "America/New_York",
        "Asia/Kolkata",
        "Mars/Olympus",
    ] {
        match name.parse::<Tz>() {
            Ok(tz) => println!("{name:<17} {}", show(instant.with_timezone(&tz))),
            Err(_) => println!("{name:<17} unknown zone"),
        }
    }

    // The same wall-clock hour maps to different UTC offsets across a DST change.
    let berlin: Tz = "Europe/Berlin".parse().unwrap();
    for day in [29, 30, 31] {
        let noon = NaiveDate::from_ymd_opt(2030, 3, day)
            .unwrap()
            .and_hms_opt(12, 0, 0)
            .unwrap();
        let local = berlin.from_local_datetime(&noon).unwrap();
        println!(
            "Berlin 2030-03-{day} 12:00 -> {}",
            local
                .with_timezone(&Utc)
                .to_rfc3339_opts(SecondsFormat::Secs, true)
        );
    }

    // Local times can be missing (spring forward) or ambiguous (fall back).
    let gap: NaiveDateTime = NaiveDate::from_ymd_opt(2030, 3, 31)
        .unwrap()
        .and_hms_opt(2, 30, 0)
        .unwrap();
    let overlap: NaiveDateTime = NaiveDate::from_ymd_opt(2030, 10, 27)
        .unwrap()
        .and_hms_opt(2, 30, 0)
        .unwrap();
    println!(
        "02:30 on 2030-03-31 (clocks jump 02:00 -> 03:00): {}",
        describe(berlin.from_local_datetime(&gap))
    );
    println!(
        "02:30 on 2030-10-27 (clocks go back 03:00 -> 02:00): {}",
        describe(berlin.from_local_datetime(&overlap))
    );
}

fn describe(r: LocalResult<DateTime<Tz>>) -> String {
    match r {
        LocalResult::Single(t) => format!("single {}", show(t)),
        LocalResult::Ambiguous(a, b) => format!("ambiguous: {} or {}", show(a), show(b)),
        LocalResult::None => "does not exist".to_string(),
    }
}
