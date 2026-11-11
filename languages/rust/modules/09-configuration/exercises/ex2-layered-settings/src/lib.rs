//! Exercise 2: layered settings: defaults < file < environment < command line.
use std::net::SocketAddr;

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

impl Layer {
    /// Parse a TOML document with the same keys as the fields. Unknown keys and wrong types
    /// are errors; return the `toml` error text.
    pub fn from_toml(_text: &str) -> Result<Layer, String> {
        todo!("toml::from_str")
    }

    /// Read `TT_ADDR`, `TT_TIMEOUT_MS`, `TT_DATABASE_URL`, `TT_WORKERS`. Unset keys stay `None`.
    /// A key that is set but does not parse is an error that names the key.
    pub fn from_lookup(_lookup: impl Fn(&str) -> Option<String>) -> Result<Layer, String> {
        todo!("parse each key that is present")
    }

    /// `self` is the lower layer: every field that `higher` sets wins, the rest is kept.
    pub fn merge(self, _higher: Layer) -> Layer {
        todo!("field-wise `higher.or(self)`")
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
        todo!("defaults, then validation")
    }
}

/// Load everything: `file` (TOML text, if a config file exists) < environment < `cli`.
/// A layer that fails to parse contributes its error to the result instead of being ignored;
/// when no layer failed, validation errors from [`Layer::resolve`] are reported.
pub fn load(
    _file: Option<&str>,
    _lookup: impl Fn(&str) -> Option<String>,
    _cli: Layer,
) -> Result<Settings, Vec<String>> {
    todo!("build the layers, merge them in precedence order, resolve")
}
