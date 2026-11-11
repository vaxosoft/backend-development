//! Exercise 2: layered settings: defaults < file < environment < command line.
use std::net::SocketAddr;
use std::str::FromStr;

use serde::Deserialize;

/// One configuration layer: every field optional, `None` means "this layer has no opinion".
#[derive(Debug, Default, Clone, PartialEq, Eq, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Layer {
    pub addr: Option<SocketAddr>,
    pub timeout_ms: Option<u64>,
    pub database_url: Option<String>,
    pub workers: Option<usize>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Settings {
    pub addr: SocketAddr,
    pub timeout_ms: u64,
    pub database_url: String,
    pub workers: usize,
}

fn read<T: FromStr>(
    lookup: &impl Fn(&str) -> Option<String>,
    key: &str,
) -> Result<Option<T>, String> {
    lookup(key)
        .map(|raw| {
            raw.trim()
                .parse()
                .map_err(|_| format!("{key}: invalid value {raw:?}"))
        })
        .transpose()
}

impl Layer {
    /// Parse a TOML document with the same keys as the fields. Unknown keys and wrong types
    /// are errors; return the `toml` error text.
    pub fn from_toml(text: &str) -> Result<Layer, String> {
        toml::from_str(text).map_err(|e| e.to_string())
    }

    /// Read `TT_ADDR`, `TT_TIMEOUT_MS`, `TT_DATABASE_URL`, `TT_WORKERS`. Unset keys stay `None`.
    /// A key that is set but does not parse is an error that names the key.
    pub fn from_lookup(lookup: impl Fn(&str) -> Option<String>) -> Result<Layer, String> {
        Ok(Layer {
            addr: read(&lookup, "TT_ADDR")?,
            timeout_ms: read(&lookup, "TT_TIMEOUT_MS")?,
            database_url: lookup("TT_DATABASE_URL"),
            workers: read(&lookup, "TT_WORKERS")?,
        })
    }

    /// `self` is the lower layer: every field that `higher` sets wins, the rest is kept.
    pub fn merge(self, higher: Layer) -> Layer {
        Layer {
            addr: higher.addr.or(self.addr),
            timeout_ms: higher.timeout_ms.or(self.timeout_ms),
            database_url: higher.database_url.or(self.database_url),
            workers: higher.workers.or(self.workers),
        }
    }

    /// Apply the defaults and validate. Report every violation:
    ///
    /// | field | default | rule |
    /// |-------|---------|------|
    /// | `addr` | `127.0.0.1:8080` | |
    /// | `timeout_ms` | `5000` | 1..=60000 |
    /// | `database_url` | `sqlite::memory:` | starts with `sqlite:` or `postgres://` |
    /// | `workers` | `4` | 1..=64 |
    ///
    /// Each message starts with the field name, for example `"timeout_ms must be 1-60000, got 0"`.
    pub fn resolve(self) -> Result<Settings, Vec<String>> {
        let addr = self
            .addr
            .unwrap_or_else(|| SocketAddr::from(([127, 0, 0, 1], 8080)));
        let timeout_ms = self.timeout_ms.unwrap_or(5_000);
        let database_url = self
            .database_url
            .unwrap_or_else(|| "sqlite::memory:".to_owned());
        let workers = self.workers.unwrap_or(4);

        let mut errors = Vec::new();
        if !(1..=60_000).contains(&timeout_ms) {
            errors.push(format!("timeout_ms must be 1-60000, got {timeout_ms}"));
        }
        if !(database_url.starts_with("sqlite:") || database_url.starts_with("postgres://")) {
            errors.push(format!(
                "database_url must start with sqlite: or postgres://, got {database_url:?}"
            ));
        }
        if !(1..=64).contains(&workers) {
            errors.push(format!("workers must be 1-64, got {workers}"));
        }
        if errors.is_empty() {
            Ok(Settings {
                addr,
                timeout_ms,
                database_url,
                workers,
            })
        } else {
            Err(errors)
        }
    }
}

/// Load everything: `file` (TOML text, if a config file exists) < environment < `cli`.
/// A layer that fails to parse contributes its error to the result instead of being ignored;
/// when no layer failed, validation errors from [`Layer::resolve`] are reported.
pub fn load(
    file: Option<&str>,
    lookup: impl Fn(&str) -> Option<String>,
    cli: Layer,
) -> Result<Settings, Vec<String>> {
    let mut errors = Vec::new();
    let file_layer = match file.map(Layer::from_toml).transpose() {
        Ok(layer) => layer.unwrap_or_default(),
        Err(error) => {
            errors.push(error);
            Layer::default()
        }
    };
    let env_layer = match Layer::from_lookup(lookup) {
        Ok(layer) => layer,
        Err(error) => {
            errors.push(error);
            Layer::default()
        }
    };
    if !errors.is_empty() {
        return Err(errors);
    }
    file_layer.merge(env_layer).merge(cli).resolve()
}
