use chrono::{DateTime, NaiveDate, TimeZone, Utc};
use m13_ex2_due_dates_and_zones::{
    TimeError, days_until, end_of_day_utc, is_overdue, local_today, parse_due_date, parse_zone,
    start_of_day_utc,
};

fn date(y: i32, m: u32, d: u32) -> NaiveDate {
    NaiveDate::from_ymd_opt(y, m, d).unwrap()
}

fn utc(y: i32, m: u32, d: u32, h: u32, mi: u32, s: u32) -> DateTime<Utc> {
    Utc.with_ymd_and_hms(y, m, d, h, mi, s).unwrap()
}

#[test]
fn parses_strict_iso_dates_only() {
    assert_eq!(parse_due_date("2030-01-31"), Ok(date(2030, 1, 31)));
    assert_eq!(parse_due_date("2032-02-29"), Ok(date(2032, 2, 29))); // leap year
    for bad in [
        "2030-1-5",
        "2030-01-5",
        " 2030-01-05",
        "2030-01-05 ",
        "2030-02-30",
        "2031-02-29",
        "2030-13-01",
        "2030-01-05T00:00",
        "30-01-05",
        "",
    ] {
        assert_eq!(
            parse_due_date(bad),
            Err(TimeError::BadDate),
            "input {bad:?}"
        );
    }
}

#[test]
fn zones_parse_or_report_the_unknown_name() {
    assert_eq!(parse_zone("Europe/Berlin").unwrap().name(), "Europe/Berlin");
    assert_eq!(
        parse_zone("Mars/Olympus"),
        Err(TimeError::UnknownZone("Mars/Olympus".to_string()))
    );
}

#[test]
fn days_until_and_overdue_use_calendar_days() {
    let today = date(2030, 3, 30);
    assert_eq!(days_until(date(2030, 3, 31), today), 1);
    assert_eq!(days_until(today, today), 0);
    assert_eq!(days_until(date(2030, 3, 25), today), -5);
    assert_eq!(days_until(date(2031, 3, 30), today), 365);

    assert!(is_overdue(date(2030, 3, 29), today));
    assert!(!is_overdue(today, today), "due today is not overdue");
    assert!(!is_overdue(date(2030, 3, 31), today));
}

#[test]
fn today_depends_on_the_zone() {
    let now = utc(2030, 1, 1, 23, 30, 0);
    assert_eq!(
        local_today(now, parse_zone("UTC").unwrap()),
        date(2030, 1, 1)
    );
    assert_eq!(
        local_today(now, parse_zone("Asia/Tokyo").unwrap()),
        date(2030, 1, 2)
    );
    assert_eq!(
        local_today(now, parse_zone("America/Los_Angeles").unwrap()),
        date(2030, 1, 1)
    );
    assert_eq!(
        local_today(
            utc(2030, 1, 1, 3, 0, 0),
            parse_zone("America/Los_Angeles").unwrap()
        ),
        date(2029, 12, 31)
    );
}

#[test]
fn an_ordinary_day_in_berlin() {
    let berlin = parse_zone("Europe/Berlin").unwrap();
    assert_eq!(
        start_of_day_utc(date(2030, 1, 15), berlin),
        utc(2030, 1, 14, 23, 0, 0)
    ); // CET = UTC+1
    assert_eq!(
        end_of_day_utc(date(2030, 1, 15), berlin),
        utc(2030, 1, 15, 22, 59, 59)
    );
    assert_eq!(
        end_of_day_utc(date(2030, 7, 15), berlin),
        utc(2030, 7, 15, 21, 59, 59)
    ); // CEST = UTC+2
}

#[test]
fn days_have_23_and_25_hours_around_dst_changes() {
    let berlin = parse_zone("Europe/Berlin").unwrap();
    let length =
        |d| end_of_day_utc(d, berlin) - start_of_day_utc(d, berlin) + chrono::Duration::seconds(1);

    assert_eq!(length(date(2030, 3, 31)).num_hours(), 23); // spring forward
    assert_eq!(length(date(2030, 10, 27)).num_hours(), 25); // fall back
    assert_eq!(length(date(2030, 6, 1)).num_hours(), 24);
}

#[test]
fn midnight_that_does_not_exist_starts_the_day_at_the_first_valid_time() {
    // Brazil started DST at local midnight on 2018-11-04: 00:00 jumped straight to 01:00 (UTC-2)
    let sao_paulo = parse_zone("America/Sao_Paulo").unwrap();
    assert_eq!(
        start_of_day_utc(date(2018, 11, 4), sao_paulo),
        utc(2018, 11, 4, 3, 0, 0)
    );
    assert_eq!(
        start_of_day_utc(date(2018, 11, 3), sao_paulo),
        utc(2018, 11, 3, 3, 0, 0)
    ); // UTC-3
}

#[test]
fn an_ambiguous_last_second_uses_the_later_instance() {
    // Lebanon ends DST at local midnight: on 2030-10-27 00:00 (UTC+3) clocks go back to 23:00 (UTC+2),
    // so Saturday 2030-10-26 23:59:59 happens twice (20:59:59Z and 21:59:59Z).
    let beirut = parse_zone("Asia/Beirut").unwrap();
    assert_eq!(
        end_of_day_utc(date(2030, 10, 26), beirut),
        utc(2030, 10, 26, 21, 59, 59)
    );
    assert_eq!(
        start_of_day_utc(date(2030, 10, 26), beirut),
        utc(2030, 10, 25, 21, 0, 0)
    );
    assert_eq!(
        start_of_day_utc(date(2030, 10, 27), beirut),
        utc(2030, 10, 26, 22, 0, 0)
    );
}
