use chrono::{DateTime, Duration, NaiveTime, TimeZone, Utc};
use m13_ex3_scheduler::{Schedule, ScheduleError, Scheduler};

fn at(d: u32, h: u32, m: u32, s: u32) -> DateTime<Utc> {
    Utc.with_ymd_and_hms(2030, 1, d, h, m, s).unwrap()
}

fn daily(h: u32, m: u32) -> Schedule {
    Schedule::DailyAt(NaiveTime::from_hms_opt(h, m, 0).unwrap())
}

#[test]
fn intervals_must_be_positive() {
    assert_eq!(
        Schedule::every(Duration::zero()),
        Err(ScheduleError::NonPositiveInterval)
    );
    assert_eq!(
        Schedule::every(Duration::seconds(-5)),
        Err(ScheduleError::NonPositiveInterval)
    );
    assert_eq!(
        Schedule::every(Duration::seconds(30)),
        Ok(Schedule::Every(Duration::seconds(30)))
    );
}

#[test]
fn every_counts_from_the_reference_time() {
    let s = Schedule::every(Duration::minutes(15)).unwrap();
    assert_eq!(s.next_after(at(1, 9, 0, 0)), at(1, 9, 15, 0));
    assert_eq!(s.next_after(at(1, 23, 50, 0)), at(2, 0, 5, 0));
}

#[test]
fn daily_picks_today_if_ahead_otherwise_tomorrow() {
    let s = daily(8, 30);
    assert_eq!(s.next_after(at(1, 7, 0, 0)), at(1, 8, 30, 0));
    assert_eq!(s.next_after(at(1, 8, 29, 59)), at(1, 8, 30, 0));
    assert_eq!(
        s.next_after(at(1, 8, 30, 0)),
        at(2, 8, 30, 0),
        "strictly after"
    );
    assert_eq!(s.next_after(at(1, 22, 0, 0)), at(2, 8, 30, 0));
    // month boundary
    assert_eq!(
        s.next_after(Utc.with_ymd_and_hms(2030, 1, 31, 12, 0, 0).unwrap()),
        Utc.with_ymd_and_hms(2030, 2, 1, 8, 30, 0).unwrap()
    );
}

#[test]
fn nothing_is_due_before_the_first_run() {
    let mut sched = Scheduler::new();
    sched.add(
        "reminders",
        Schedule::every(Duration::minutes(10)).unwrap(),
        at(1, 9, 0, 0),
    );
    assert_eq!(sched.len(), 1);
    assert_eq!(sched.next_wakeup(), Some(at(1, 9, 10, 0)));
    assert!(sched.due(at(1, 9, 9, 59)).is_empty());
    assert_eq!(sched.due(at(1, 9, 10, 0)), ["reminders"]);
}

#[test]
fn a_job_runs_again_after_its_interval() {
    let mut sched = Scheduler::new();
    sched.add(
        "sync",
        Schedule::every(Duration::minutes(10)).unwrap(),
        at(1, 9, 0, 0),
    );
    assert_eq!(sched.due(at(1, 9, 10, 0)), ["sync"]);
    assert!(sched.due(at(1, 9, 19, 0)).is_empty());
    assert_eq!(sched.next_wakeup(), Some(at(1, 9, 20, 0)));
    assert_eq!(sched.due(at(1, 9, 20, 0)), ["sync"]);
}

#[test]
fn missed_runs_are_skipped_not_replayed() {
    let mut sched = Scheduler::new();
    sched.add(
        "sync",
        Schedule::every(Duration::minutes(10)).unwrap(),
        at(1, 9, 0, 0),
    );
    // the process was suspended for two hours
    assert_eq!(
        sched.due(at(1, 11, 0, 0)),
        ["sync"],
        "exactly one run, not twelve"
    );
    assert!(sched.due(at(1, 11, 0, 1)).is_empty());
    assert_eq!(sched.next_wakeup(), Some(at(1, 11, 10, 0)));
}

#[test]
fn daily_job_after_a_long_outage_runs_once_and_waits_for_the_next_slot() {
    let mut sched = Scheduler::new();
    sched.add("digest", daily(8, 0), at(1, 12, 0, 0));
    assert_eq!(sched.next_wakeup(), Some(at(2, 8, 0, 0)));

    assert_eq!(sched.due(at(5, 3, 0, 0)), ["digest"]);
    assert_eq!(sched.next_wakeup(), Some(at(5, 8, 0, 0)));
    assert_eq!(sched.due(at(5, 8, 0, 0)), ["digest"]);
    assert_eq!(sched.next_wakeup(), Some(at(6, 8, 0, 0)));
}

#[test]
fn due_jobs_come_back_in_registration_order_and_the_wakeup_is_the_earliest() {
    let mut sched = Scheduler::new();
    assert_eq!(sched.next_wakeup(), None);
    assert!(sched.is_empty());

    let start = at(1, 9, 0, 0);
    sched.add("slow", Schedule::every(Duration::hours(1)).unwrap(), start);
    sched.add(
        "fast",
        Schedule::every(Duration::minutes(1)).unwrap(),
        start,
    );
    sched.add("daily", daily(9, 1), start);

    assert_eq!(sched.next_wakeup(), Some(at(1, 9, 1, 0)));
    assert_eq!(sched.due(at(1, 9, 1, 0)), ["fast", "daily"]);
    assert_eq!(sched.due(at(1, 10, 0, 0)), ["slow", "fast"]);
}
