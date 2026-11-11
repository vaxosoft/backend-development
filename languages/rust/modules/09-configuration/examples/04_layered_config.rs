//! Layered configuration: defaults < config file < environment < command line.
//! Each layer is a `Partial` (every field optional); layers merge, then one step validates.
use std::net::SocketAddr;
use std::time::Duration;

use clap::{Parser, ValueEnum};
use serde::Deserialize;

#[derive(Debug, Clone, Copy, PartialEq, Deserialize, ValueEnum)]
#[serde(rename_all = "lowercase")]
enum Level {
    Debug,
    Info,
    Warn,
}

/// One layer. The same struct is filled from TOML (serde) and from flags (clap).
#[derive(Debug, Default, Clone, Deserialize, Parser)]
#[serde(deny_unknown_fields)]
struct Partial {
    #[arg(long)]
    addr: Option<SocketAddr>,
    #[arg(long)]
    timeout_ms: Option<u64>,
    #[arg(long)]
    database_url: Option<String>,
    #[arg(long, value_enum)]
    log_level: Option<Level>,
}

impl Partial {
    fn from_toml(text: &str) -> Result<Self, toml::de::Error> {
        toml::from_str(text)
    }

    fn from_env(lookup: impl Fn(&str) -> Option<String>) -> Result<Self, String> {
        fn get<T: std::str::FromStr>(
            lookup: &impl Fn(&str) -> Option<String>,
            key: &str,
        ) -> Result<Option<T>, String> {
            lookup(key)
                .map(|raw| {
                    raw.parse()
                        .map_err(|_| format!("{key}={raw:?} is not valid"))
                })
                .transpose()
        }
        Ok(Self {
            addr: get(&lookup, "TT_ADDR")?,
            timeout_ms: get(&lookup, "TT_TIMEOUT_MS")?,
            database_url: get(&lookup, "TT_DATABASE_URL")?,
            log_level: lookup("TT_LOG_LEVEL")
                .map(|raw| Level::from_str(&raw, true).map_err(|e| format!("TT_LOG_LEVEL: {e}")))
                .transpose()?,
        })
    }

    /// Values in `higher` win; anything it leaves unset is kept from `self`.
    fn overridden_by(self, higher: Partial) -> Partial {
        Partial {
            addr: higher.addr.or(self.addr),
            timeout_ms: higher.timeout_ms.or(self.timeout_ms),
            database_url: higher.database_url.or(self.database_url),
            log_level: higher.log_level.or(self.log_level),
        }
    }

    /// Apply defaults and validate, reporting every problem at once.
    fn finish(self) -> Result<Config, Vec<String>> {
        let mut errors = Vec::new();
        let timeout_ms = self.timeout_ms.unwrap_or(5_000);
        if !(1..=60_000).contains(&timeout_ms) {
            errors.push(format!("timeout_ms must be 1-60000, got {timeout_ms}"));
        }
        let database_url = self
            .database_url
            .unwrap_or_else(|| "sqlite::memory:".to_owned());
        if !(database_url.starts_with("sqlite:") || database_url.starts_with("postgres://")) {
            errors.push(format!(
                "database_url must start with sqlite: or postgres://, got {database_url:?}"
            ));
        }
        if !errors.is_empty() {
            return Err(errors);
        }
        Ok(Config {
            addr: self
                .addr
                .unwrap_or_else(|| "127.0.0.1:8080".parse().expect("valid literal")),
            request_timeout: Duration::from_millis(timeout_ms),
            database_url,
            log_level: self.log_level.unwrap_or(Level::Info),
        })
    }
}

#[derive(Debug)]
struct Config {
    addr: SocketAddr,
    request_timeout: Duration,
    database_url: String,
    log_level: Level,
}

fn show(label: &str, result: Result<Config, Vec<String>>) {
    match result {
        Ok(c) => println!(
            "{label:<13} {} {:?} {} {:?}",
            c.addr, c.request_timeout, c.database_url, c.log_level
        ),
        Err(errors) => println!("{label:<13} rejected: {}", errors.join("; ")),
    }
}

fn main() -> anyhow::Result<()> {
    let file = Partial::from_toml(
        "addr = \"0.0.0.0:9000\"\ndatabase_url = \"sqlite://tasks.db\"\nlog_level = \"warn\"\n",
    )?;
    let env = |key: &str| match key {
        "TT_ADDR" => Some("0.0.0.0:7000".to_owned()),
        "TT_TIMEOUT_MS" => Some("250".to_owned()),
        _ => None,
    };
    let flags = Partial::try_parse_from(["tasktracker", "--addr", "127.0.0.1:1234"])?;

    let mut layers = Partial::default();
    show("defaults:", layers.clone().finish());
    layers = layers.overridden_by(file);
    show("+ file:", layers.clone().finish());
    layers = layers.overridden_by(Partial::from_env(env).map_err(anyhow::Error::msg)?);
    show("+ env:", layers.clone().finish());
    layers = layers.overridden_by(flags);
    show("+ flags:", layers.clone().finish());

    let invalid = Partial {
        timeout_ms: Some(0),
        database_url: Some("mysql://db".into()),
        ..Partial::default()
    };
    show("bad values:", layers.overridden_by(invalid).finish());

    let bad_env = |key: &str| (key == "TT_TIMEOUT_MS").then(|| "soon".to_owned());
    println!("bad env:      {}", Partial::from_env(bad_env).unwrap_err());
    Ok(())
}
