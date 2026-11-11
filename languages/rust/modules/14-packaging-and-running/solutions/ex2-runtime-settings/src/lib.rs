//! Exercise 2: resolve runtime settings from flags, environment and defaults, then validate them.
//!
//! Precedence, highest first: command-line flag, environment variable, built-in default.
//! The environment is a parameter (not `std::env`) so tests are deterministic and parallel-safe.

use std::collections::HashMap;
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

/// (flag name, environment variable)
const OPTIONS: [(&str, &str); 5] = [
    ("env", "APP_ENV"),
    ("addr", "ADDR"),
    ("log-level", "LOG_LEVEL"),
    ("workers", "WORKERS"),
    ("shutdown-timeout-secs", "SHUTDOWN_TIMEOUT_SECS"),
];

fn parse_flags(args: &[&str]) -> Result<HashMap<&'static str, String>, SettingsError> {
    let mut flags = HashMap::new();
    let mut iter = args.iter();
    while let Some(arg) = iter.next() {
        let Some(rest) = arg.strip_prefix("--") else {
            return Err(SettingsError::UnknownFlag((*arg).to_string()));
        };
        let (name, inline) = match rest.split_once('=') {
            Some((name, value)) => (name, Some(value.to_string())),
            None => (rest, None),
        };
        let Some((key, _)) = OPTIONS.iter().find(|(flag, _)| *flag == name) else {
            return Err(SettingsError::UnknownFlag((*arg).to_string()));
        };
        let value = match inline {
            Some(v) => v,
            None => iter
                .next()
                .map(|v| (*v).to_string())
                .ok_or_else(|| SettingsError::MissingValue((*arg).to_string()))?,
        };
        flags.insert(*key, value);
    }
    Ok(flags)
}

fn invalid(field: &'static str, value: &str, reason: &'static str) -> SettingsError {
    SettingsError::Invalid {
        field,
        value: value.to_string(),
        reason,
    }
}

fn ranged(field: &'static str, value: &str, max: u64) -> Result<u64, SettingsError> {
    let n: u64 = value
        .parse()
        .map_err(|_| invalid(field, value, "not a whole number"))?;
    if (1..=max).contains(&n) {
        Ok(n)
    } else {
        Err(invalid(field, value, "out of range"))
    }
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
    args: &[&str],
    env: &dyn Fn(&str) -> Option<String>,
) -> Result<Settings, SettingsError> {
    let flags = parse_flags(args)?;
    let lookup = |flag: &'static str| -> Option<String> {
        flags.get(flag).cloned().or_else(|| {
            let var = OPTIONS.iter().find(|(f, _)| *f == flag).map(|(_, v)| *v)?;
            env(var)
        })
    };

    let environment = match lookup("env").as_deref() {
        None | Some("development") => Environment::Development,
        Some("production") => Environment::Production,
        Some(other) => return Err(invalid("env", other, "expected development or production")),
    };

    let addr = match lookup("addr") {
        None => SocketAddr::from(([127, 0, 0, 1], 8080)),
        Some(v) => v
            .parse()
            .map_err(|_| invalid("addr", &v, "expected host:port"))?,
    };

    let log_level = match lookup("log-level").as_deref() {
        None if environment == Environment::Production => LogLevel::Info,
        None => LogLevel::Debug,
        Some("debug") => LogLevel::Debug,
        Some("info") => LogLevel::Info,
        Some("warn") => LogLevel::Warn,
        Some("error") => LogLevel::Error,
        Some(other) => {
            return Err(invalid(
                "log-level",
                other,
                "expected debug, info, warn or error",
            ));
        }
    };

    let workers = match lookup("workers") {
        None => 4,
        Some(v) => ranged("workers", &v, 256)? as usize,
    };
    let shutdown_timeout = match lookup("shutdown-timeout-secs") {
        None => Duration::from_secs(10),
        Some(v) => Duration::from_secs(ranged("shutdown-timeout-secs", &v, 300)?),
    };

    if environment == Environment::Production && log_level == LogLevel::Debug {
        return Err(SettingsError::Production("log level debug is not allowed"));
    }
    Ok(Settings {
        addr,
        log_level,
        workers,
        shutdown_timeout,
        environment,
    })
}
