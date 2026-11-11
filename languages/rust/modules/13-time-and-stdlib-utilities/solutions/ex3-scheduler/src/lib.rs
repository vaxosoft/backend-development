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
    pub fn every(interval: Duration) -> Result<Self, ScheduleError> {
        if interval <= Duration::zero() {
            return Err(ScheduleError::NonPositiveInterval);
        }
        Ok(Self::Every(interval))
    }

    /// The first run strictly after `after`. For `DailyAt` that is today's slot if it is still ahead,
    /// otherwise tomorrow's.
    pub fn next_after(&self, after: DateTime<Utc>) -> DateTime<Utc> {
        match self {
            Self::Every(interval) => after + *interval,
            Self::DailyAt(time) => {
                let today = after.date_naive().and_time(*time).and_utc();
                if today > after {
                    today
                } else {
                    today + Duration::days(1)
                }
            }
        }
    }
}

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
    pub fn add(&mut self, name: &str, schedule: Schedule, now: DateTime<Utc>) {
        self.jobs.push(Job {
            name: name.to_string(),
            schedule,
            next_run: schedule.next_after(now),
        });
    }

    /// Names of the jobs whose `next_run <= now`, in registration order. Each returned job is
    /// rescheduled to `schedule.next_after(now)`: missed runs are skipped, never replayed in a burst.
    pub fn due(&mut self, now: DateTime<Utc>) -> Vec<String> {
        let mut due = Vec::new();
        for job in self.jobs.iter_mut().filter(|j| j.next_run <= now) {
            job.next_run = job.schedule.next_after(now);
            due.push(job.name.clone());
        }
        due
    }

    /// The earliest upcoming run, i.e. how long the runner may sleep.
    pub fn next_wakeup(&self) -> Option<DateTime<Utc>> {
        self.jobs.iter().map(|j| j.next_run).min()
    }

    pub fn len(&self) -> usize {
        self.jobs.len()
    }

    pub fn is_empty(&self) -> bool {
        self.jobs.is_empty()
    }
}
