//! Read environment variables once, at startup, into a typed struct.
//! The lookup is injected, so the same code runs against the real environment or a map in a test.
use std::collections::HashMap;
use std::fmt::Display;
use std::net::SocketAddr;
use std::str::FromStr;
use std::time::Duration;

#[derive(Debug)]
struct Config {
    listen_addr: SocketAddr,
    request_timeout: Duration,
    max_page_size: usize,
}

#[derive(Debug, thiserror::Error)]
#[error("{key}={value:?}: {reason}")]
struct ConfigError {
    key: &'static str,
    value: String,
    reason: String,
}

impl Config {
    /// Collects every problem instead of stopping at the first one.
    fn from_lookup(lookup: impl Fn(&str) -> Option<String>) -> Result<Self, Vec<ConfigError>> {
        let mut errors = Vec::new();
        let listen_addr = parse(
            &lookup,
            "TT_ADDR",
            "127.0.0.1:8080".parse().expect("valid literal"),
            &mut errors,
        );
        let timeout_ms: u64 = parse(&lookup, "TT_TIMEOUT_MS", 5_000, &mut errors);
        let max_page_size = parse(&lookup, "TT_MAX_PAGE", 100, &mut errors);
        if max_page_size == 0 {
            errors.push(ConfigError {
                key: "TT_MAX_PAGE",
                value: "0".into(),
                reason: "must be at least 1".into(),
            });
        }
        if !errors.is_empty() {
            return Err(errors);
        }
        Ok(Self {
            listen_addr,
            request_timeout: Duration::from_millis(timeout_ms),
            max_page_size,
        })
    }
}

fn parse<T>(
    lookup: &impl Fn(&str) -> Option<String>,
    key: &'static str,
    default: T,
    errors: &mut Vec<ConfigError>,
) -> T
where
    T: FromStr,
    T::Err: Display,
{
    let Some(raw) = lookup(key) else {
        return default;
    };
    raw.trim().parse().unwrap_or_else(|error: T::Err| {
        errors.push(ConfigError {
            key,
            value: raw.clone(),
            reason: error.to_string(),
        });
        default
    })
}

fn from_pairs(pairs: &[(&str, &str)]) -> impl Fn(&str) -> Option<String> {
    let map: HashMap<String, String> = pairs
        .iter()
        .map(|(k, v)| (k.to_string(), v.to_string()))
        .collect();
    move |key| map.get(key).cloned()
}

fn show(label: &str, config: &Config) {
    println!(
        "{label} addr={} timeout={:?} max_page={}",
        config.listen_addr, config.request_timeout, config.max_page_size
    );
}

fn main() {
    show("defaults: ", &Config::from_lookup(from_pairs(&[])).unwrap());

    let overrides = from_pairs(&[("TT_ADDR", "0.0.0.0:9000"), ("TT_TIMEOUT_MS", " 250 ")]);
    show("overrides:", &Config::from_lookup(overrides).unwrap());

    let broken = from_pairs(&[
        ("TT_ADDR", "localhost"),
        ("TT_TIMEOUT_MS", "-1"),
        ("TT_MAX_PAGE", "0"),
    ]);
    for error in Config::from_lookup(broken).unwrap_err() {
        println!("invalid:   {error}");
    }

    // The real environment: `std::env::var` distinguishes "not set" from "not valid Unicode".
    println!("TT_ADDR in this process: {:?}", std::env::var("TT_ADDR"));
}
