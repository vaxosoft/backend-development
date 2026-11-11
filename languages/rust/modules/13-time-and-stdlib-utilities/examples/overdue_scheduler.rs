//! Task Tracker: an in-process background job on `tokio::time::interval`, stopped with a CancellationToken.

use std::time::Duration;

use chrono::{Days, NaiveDate};
use tokio::time::{Instant, MissedTickBehavior, interval};
use tokio_util::sync::CancellationToken;

struct Task {
    title: &'static str,
    due: NaiveDate,
}

/// Runs `job(run_number)` every `period` until cancelled. Returns how many times it ran.
async fn run_every(period: Duration, cancel: CancellationToken, mut job: impl FnMut(u32)) -> u32 {
    let mut ticker = interval(period);
    // If a run takes longer than the period, do not fire a burst of catch-up runs.
    ticker.set_missed_tick_behavior(MissedTickBehavior::Skip);
    let mut runs = 0;
    loop {
        tokio::select! {
            _ = ticker.tick() => {
                runs += 1;
                job(runs);
            }
            _ = cancel.cancelled() => return runs,
        }
    }
}

#[tokio::main]
async fn main() {
    let tasks = [
        Task {
            title: "Pay invoices",
            due: NaiveDate::from_ymd_opt(2030, 1, 10).unwrap(),
        },
        Task {
            title: "Renew domain",
            due: NaiveDate::from_ymd_opt(2030, 1, 12).unwrap(),
        },
        Task {
            title: "Write docs",
            due: NaiveDate::from_ymd_opt(2030, 1, 20).unwrap(),
        },
    ];
    // The job works on a pretend "today" that advances one day per run, so the output is stable.
    let first_day = NaiveDate::from_ymd_opt(2030, 1, 11).unwrap();

    let period = Duration::from_millis(40);
    let started = Instant::now();
    let cancel = CancellationToken::new();
    let stop = cancel.clone();

    let handle = tokio::spawn(run_every(period, cancel, move |run| {
        let today = first_day + Days::new(u64::from(run) - 1);
        let overdue: Vec<&str> = tasks
            .iter()
            .filter(|t| t.due < today)
            .map(|t| t.title)
            .collect();
        if run == 1 {
            // interval's first tick completes immediately
            println!("first tick immediate: {}", started.elapsed() < period / 2);
        }
        println!("run {run} ({today}): overdue = {overdue:?}");
        if run == 3 {
            stop.cancel();
        }
    }));

    let runs = handle.await.expect("job task panicked");
    println!(
        "stopped after {runs} runs; at least two periods elapsed: {}",
        started.elapsed() >= period * 2
    );
}
