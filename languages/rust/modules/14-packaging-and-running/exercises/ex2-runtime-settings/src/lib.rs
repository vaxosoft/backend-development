//! Exercise 2: resolve runtime settings from flags, environment and defaults, then validate them.
//!
//! Precedence, highest first: command-line flag, environment variable, built-in default.
//! The environment is a parameter (not `std::env`) so tests are deterministic and parallel-safe.

use std::net::SocketAddr;
use std::time::Duration;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum LogLevel {
    Debug,
    Info,
    Warn,
    Error,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Environment {
    Development,
    Production,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Settings {
    pub addr: SocketAddr,
    pub log_level: LogLevel,
    pub workers: usize,
    pub shutdown_timeout: Duration,
    pub environment: Environment,
}

#[derive(Debug, PartialEq, Eq, thiserror::Error)]
pub enum SettingsError {
    #[error("unknown flag {0}")]
    UnknownFlag(String),
    #[error("flag {0} needs a value")]
    MissingValue(String),
    #[error("invalid {field}: {value:?} ({reason})")]
    Invalid {
        field: &'static str,
        value: String,
        reason: &'static str,
    },
    #[error("production rule violated: {0}")]
    Production(&'static str),
}

/// | flag                      | env                     | default (dev / prod)        | valid                         |
/// |---------------------------|-------------------------|-----------------------------|-------------------------------|
/// | `--env`                   | `APP_ENV`               | `development`               | `development`, `production`   |
/// | `--addr`                  | `ADDR`                  | `127.0.0.1:8080`            | a socket address              |
/// | `--log-level`             | `LOG_LEVEL`             | `debug` / `info`            | `debug`, `info`, `warn`, `error` |
/// | `--workers`               | `WORKERS`               | `4`                         | 1 to 256                      |
/// | `--shutdown-timeout-secs` | `SHUTDOWN_TIMEOUT_SECS` | `10`                        | 1 to 300                      |
///
/// Flags are `--name value` or `--name=value`; a repeated flag keeps the last value.
/// In production the log level may not be `debug`; this is checked after precedence is applied.
/// Errors: unknown flag, flag without value, a value that does not parse or is out of range
/// (`Invalid { field, value, reason }`, where `field` is the flag name without dashes, such as `"workers"`).
pub fn resolve(
    _args: &[&str],
    _env: &dyn Fn(&str) -> Option<String>,
) -> Result<Settings, SettingsError> {
    todo!("parse flags, fall back to env, then defaults; validate; apply the production rule")
}
