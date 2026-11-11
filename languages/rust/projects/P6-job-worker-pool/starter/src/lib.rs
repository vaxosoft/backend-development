//! P6: a job worker pool living inside an HTTP service.
//!
//! * [`jobs`]: job types, validation and the JSON view (given)
//! * [`http`]: routes, handlers and the error envelope (given)
//! * [`exec`]: what each job type does (your work)
//! * [`pool`]: queue, workers, retries, cancellation, drain (your work)

pub mod exec;
pub mod http;
pub mod jobs;
pub mod pool;

use std::future::Future;
use std::io;
use std::time::Duration;

use axum::Router;
use tokio::net::TcpListener;

pub use pool::Pool;

#[derive(Debug, Clone)]
pub struct Config {
    pub workers: usize,
    pub queue_capacity: usize,
    pub max_attempts: u32,
    pub retry_delay: Duration,
    pub job_timeout: Duration,
    pub drain_timeout: Duration,
}

impl Default for Config {
    fn default() -> Self {
        Self {
            workers: 4,
            queue_capacity: 64,
            max_attempts: 3,
            retry_delay: Duration::from_millis(200),
            job_timeout: Duration::from_secs(10),
            drain_timeout: Duration::from_secs(10),
        }
    }
}

impl Config {
    /// Defaults overridden by `WORKERS`, `QUEUE_CAPACITY`, `MAX_ATTEMPTS`,
    /// `RETRY_DELAY_MS`, `JOB_TIMEOUT_MS` and `DRAIN_TIMEOUT_MS`.
    pub fn from_env() -> Result<Self, String> {
        fn var<T: std::str::FromStr>(name: &str, default: T) -> Result<T, String> {
            match std::env::var(name) {
                Ok(raw) => raw.parse().map_err(|_| format!("invalid {name}: {raw:?}")),
                Err(_) => Ok(default),
            }
        }
        let d = Self::default();
        let millis = |name: &str, default: Duration| {
            var(name, default.as_millis() as u64).map(Duration::from_millis)
        };
        Ok(Self {
            workers: var("WORKERS", d.workers)?,
            queue_capacity: var("QUEUE_CAPACITY", d.queue_capacity)?,
            max_attempts: var("MAX_ATTEMPTS", d.max_attempts)?,
            retry_delay: millis("RETRY_DELAY_MS", d.retry_delay)?,
            job_timeout: millis("JOB_TIMEOUT_MS", d.job_timeout)?,
            drain_timeout: millis("DRAIN_TIMEOUT_MS", d.drain_timeout)?,
        })
    }
}

/// Starts a pool with `cfg` and returns the HTTP router in front of it.
/// Must be called inside a Tokio runtime because the workers are spawned here.
pub fn build_app(cfg: Config) -> Router {
    http::router(Pool::start(cfg))
}

/// Serves `pool` on `listener` until `shutdown` completes, then drains the pool:
/// no new connections are accepted, in-flight requests finish, and queued or
/// running jobs get `Config::drain_timeout` to finish.
pub async fn run(
    listener: TcpListener,
    pool: Pool,
    shutdown: impl Future<Output = ()> + Send + 'static,
) -> io::Result<()> {
    axum::serve(listener, http::router(pool.clone()))
        .with_graceful_shutdown(shutdown)
        .await?;
    pool.drain().await;
    Ok(())
}
