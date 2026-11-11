//! Exercise 1: typed configuration from environment-style key/value lookups.
use std::fmt;
use std::net::SocketAddr;
use std::time::Duration;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Level {
    Error,
    Warn,
    Info,
    Debug,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct AppConfig {
    pub addr: SocketAddr,
    pub database_url: String,
    pub log_level: Level,
    pub request_timeout: Duration,
    pub signup_enabled: bool,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ConfigError {
    pub key: &'static str,
    pub message: String,
}

impl fmt::Display for ConfigError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}: {}", self.key, self.message)
    }
}

/// `"500ms"`, `"30s"`, `"2m"`: a non-negative integer followed by `ms`, `s` or `m`.
/// Anything else (no unit, unknown unit, negative, empty, `"1.5s"`) is an `Err` with a message.
pub fn parse_duration(_input: &str) -> Result<Duration, String> {
    todo!("split the digits from the unit")
}

/// `true/false`, `1/0`, `yes/no`, `on/off`, case-insensitive.
pub fn parse_bool(_input: &str) -> Result<bool, String> {
    todo!("accept the common spellings, reject the rest")
}

impl AppConfig {
    /// Build the config from a key lookup (`std::env::var(k).ok()` in production).
    ///
    /// | key | type | default |
    /// |-----|------|---------|
    /// | `TT_ADDR` | `SocketAddr` | `127.0.0.1:8080` |
    /// | `TT_DATABASE_URL` | string | REQUIRED |
    /// | `TT_LOG_LEVEL` | `error`/`warn`/`info`/`debug` (any case) | `info` |
    /// | `TT_REQUEST_TIMEOUT` | [`parse_duration`] | `30s` |
    /// | `TT_SIGNUP` | [`parse_bool`] | `false` |
    ///
    /// Values are trimmed; a blank value counts as "not set". Report ALL problems at once,
    /// in the order of the table above.
    pub fn from_lookup(_lookup: impl Fn(&str) -> Option<String>) -> Result<Self, Vec<ConfigError>> {
        todo!("read each key, apply defaults, collect errors")
    }
}
