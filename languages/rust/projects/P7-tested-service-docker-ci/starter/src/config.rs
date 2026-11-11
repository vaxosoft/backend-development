//! Environment configuration with validation.

use thiserror::Error;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum LogLevel {
    Debug,
    Info,
    Warn,
    Error,
}

impl From<LogLevel> for tracing::Level {
    fn from(level: LogLevel) -> Self {
        match level {
            LogLevel::Debug => tracing::Level::DEBUG,
            LogLevel::Info => tracing::Level::INFO,
            LogLevel::Warn => tracing::Level::WARN,
            LogLevel::Error => tracing::Level::ERROR,
        }
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Environment {
    Dev,
    Staging,
    Prod,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Config {
    pub port: u16,
    pub log_level: LogLevel,
    pub env: Environment,
}

impl Default for Config {
    fn default() -> Self {
        Self {
            port: 8080,
            log_level: LogLevel::Info,
            env: Environment::Dev,
        }
    }
}

/// A configuration variable with an unusable value.
#[derive(Debug, Error, PartialEq, Eq)]
#[error("invalid {var}={value:?}: expected {expected}")]
pub struct ConfigError {
    pub var: &'static str,
    pub value: String,
    pub expected: &'static str,
}

impl Config {
    pub fn from_env() -> Result<Self, ConfigError> {
        Self::from_lookup(|name| std::env::var(name).ok())
    }

    /// Reads `PORT` (1-65535, default 8080), `LOG_LEVEL` (`debug|info|warn|error`,
    /// default `info`) and `ENV` (`dev|staging|prod`, default `dev`).
    ///
    /// `lookup` stands in for the process environment so tests never touch it.
    /// An unset variable means "use the default"; a set but invalid one is an
    /// error naming the variable, the bad value and what was expected.
    pub fn from_lookup(_lookup: impl Fn(&str) -> Option<String>) -> Result<Self, ConfigError> {
        // TODO: validate the three variables instead of returning the defaults.
        Ok(Self::default())
    }

    /// Where the service listens when `ADDR` is not set: all interfaces, so a container works.
    pub fn listen_addr(&self) -> String {
        format!("0.0.0.0:{}", self.port)
    }
}
