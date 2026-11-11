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
pub fn parse_duration(input: &str) -> Result<Duration, String> {
    let input = input.trim();
    let split = input
        .find(|c: char| !c.is_ascii_digit())
        .unwrap_or(input.len());
    let (digits, unit) = input.split_at(split);
    let amount: u64 = digits
        .parse()
        .map_err(|_| format!("{input:?} does not start with a whole number"))?;
    match unit {
        "ms" => Ok(Duration::from_millis(amount)),
        "s" => Ok(Duration::from_secs(amount)),
        "m" => Ok(Duration::from_secs(amount * 60)),
        _ => Err(format!("{input:?} needs a unit: ms, s or m")),
    }
}

/// `true/false`, `1/0`, `yes/no`, `on/off`, case-insensitive.
pub fn parse_bool(input: &str) -> Result<bool, String> {
    match input.trim().to_ascii_lowercase().as_str() {
        "true" | "1" | "yes" | "on" => Ok(true),
        "false" | "0" | "no" | "off" => Ok(false),
        _ => Err(format!(
            "{input:?} is not a boolean (use true/false, 1/0, yes/no, on/off)"
        )),
    }
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
    pub fn from_lookup(lookup: impl Fn(&str) -> Option<String>) -> Result<Self, Vec<ConfigError>> {
        let get = |key: &str| {
            lookup(key)
                .map(|v| v.trim().to_owned())
                .filter(|v| !v.is_empty())
        };
        let mut errors = Vec::new();
        let mut fail =
            |key: &'static str, message: String| errors.push(ConfigError { key, message });

        let addr = match get("TT_ADDR") {
            None => Some(SocketAddr::from(([127, 0, 0, 1], 8080))),
            Some(raw) => raw
                .parse()
                .map_err(|e| fail("TT_ADDR", format!("{raw:?}: {e}")))
                .ok(),
        };
        let database_url = get("TT_DATABASE_URL");
        if database_url.is_none() {
            fail("TT_DATABASE_URL", "is required".into());
        }
        let log_level = match get("TT_LOG_LEVEL").map(|raw| (raw.to_ascii_lowercase(), raw)) {
            None => Some(Level::Info),
            Some((lower, raw)) => match lower.as_str() {
                "error" => Some(Level::Error),
                "warn" => Some(Level::Warn),
                "info" => Some(Level::Info),
                "debug" => Some(Level::Debug),
                _ => {
                    fail(
                        "TT_LOG_LEVEL",
                        format!("{raw:?} is not one of error, warn, info, debug"),
                    );
                    None
                }
            },
        };
        let request_timeout = match get("TT_REQUEST_TIMEOUT") {
            None => Some(Duration::from_secs(30)),
            Some(raw) => parse_duration(&raw)
                .map_err(|m| fail("TT_REQUEST_TIMEOUT", m))
                .ok(),
        };
        let signup_enabled = match get("TT_SIGNUP") {
            None => Some(false),
            Some(raw) => parse_bool(&raw).map_err(|m| fail("TT_SIGNUP", m)).ok(),
        };

        match (
            addr,
            database_url,
            log_level,
            request_timeout,
            signup_enabled,
        ) {
            (
                Some(addr),
                Some(database_url),
                Some(log_level),
                Some(request_timeout),
                Some(signup_enabled),
            ) => Ok(Self {
                addr,
                database_url,
                log_level,
                request_timeout,
                signup_enabled,
            }),
            _ => Err(errors),
        }
    }
}
