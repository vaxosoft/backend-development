//! Custom error enums with `thiserror`, `?`, `From` and `source()`.

use std::error::Error;
use std::num::ParseIntError;

#[derive(Debug, thiserror::Error)]
enum ConfigError {
    #[error("missing key '{0}'")]
    Missing(&'static str),
    #[error("invalid port '{value}'")]
    InvalidPort {
        value: String,
        #[source]
        source: ParseIntError,
    },
    #[error("port {0} is reserved")]
    Reserved(u16),
}

#[derive(Debug, thiserror::Error)]
enum StartupError {
    // `#[from]` generates `From<ConfigError>` so `?` converts automatically.
    #[error("bad configuration")]
    Config(#[from] ConfigError),
}

fn lookup<'a>(pairs: &[(&'a str, &'a str)], key: &'static str) -> Result<&'a str, ConfigError> {
    pairs
        .iter()
        .find(|(k, _)| *k == key)
        .map(|(_, v)| *v)
        .ok_or(ConfigError::Missing(key))
}

fn parse_port(pairs: &[(&str, &str)]) -> Result<u16, ConfigError> {
    let raw = lookup(pairs, "port")?;
    let port = raw
        .parse::<u16>()
        .map_err(|source| ConfigError::InvalidPort {
            value: raw.to_string(),
            source,
        })?;
    if port < 1024 {
        return Err(ConfigError::Reserved(port));
    }
    Ok(port)
}

fn start(pairs: &[(&str, &str)]) -> Result<String, StartupError> {
    let port = parse_port(pairs)?;
    Ok(format!("listening on :{port}"))
}

fn report(err: &dyn Error) {
    println!("error: {err}");
    let mut source = err.source();
    while let Some(cause) = source {
        println!("  caused by: {cause}");
        source = cause.source();
    }
}

fn main() {
    let cases: [&[(&str, &str)]; 4] = [
        &[("port", "8080")],
        &[],
        &[("port", "http")],
        &[("port", "80")],
    ];
    for pairs in cases {
        match start(pairs) {
            Ok(message) => println!("ok: {message}"),
            Err(err) => report(&err),
        }
    }
}
