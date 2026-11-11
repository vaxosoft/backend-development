//! Exercise 3: the pure logic of an in-process job scheduler. Time is a parameter, so no sleeping.
//!
//! A runner loop would be: `loop { for job in scheduler.due(now) { run(job) }; sleep until next_wakeup() }`.

use chrono::{DateTime, Duration, NaiveTime, Utc};

#[derive(Debug, PartialEq, Eq, thiserror::Error)]
pub enum ScheduleError {
    #[error("interval must be positive")]
    NonPositiveInterval,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Schedule {
    /// Repeats `interval` after the previous run (or registration).
    Every(Duration),
    /// Once a day at this UTC wall-clock time.
    DailyAt(NaiveTime),
}

impl Schedule {
    /// `Err(NonPositiveInterval)` for zero or negative intervals.
    pub fn every(_interval: Duration) -> Result<Self, ScheduleError> {
        todo!("validate the interval")
    }

    /// The first run strictly after `after`. For `DailyAt` that is today's slot if it is still ahead,
    /// otherwise tomorrow's.
    pub fn next_after(&self, _after: DateTime<Utc>) -> DateTime<Utc> {
        todo!("Every: after + interval; DailyAt: today at the time, or tomorrow")
    }
}

#[allow(dead_code)] // the fields are used by your implementation
struct Job {
    name: String,
    schedule: Schedule,
    next_run: DateTime<Utc>,
}

#[derive(Default)]
pub struct Scheduler {
    jobs: Vec<Job>,
}

impl Scheduler {
    pub fn new() -> Self {
        Self::default()
    }

    /// Registers a job; its first run is `schedule.next_after(now)`.
    pub fn add(&mut self, _name: &str, _schedule: Schedule, _now: DateTime<Utc>) {
        todo!()
    }

    /// Names of the jobs whose `next_run <= now`, in registration order. Each returned job is
    /// rescheduled to `schedule.next_after(now)`: missed runs are skipped, never replayed in a burst.
    pub fn due(&mut self, _now: DateTime<Utc>) -> Vec<String> {
        todo!()
    }

    /// The earliest upcoming run, i.e. how long the runner may sleep.
    pub fn next_wakeup(&self) -> Option<DateTime<Utc>> {
        todo!()
    }

    pub fn len(&self) -> usize {
        self.jobs.len()
    }

    pub fn is_empty(&self) -> bool {
        self.jobs.is_empty()
    }
}
