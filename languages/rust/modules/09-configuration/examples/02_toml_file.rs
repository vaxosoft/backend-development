//! A TOML config file parsed into typed sections with serde.
//! Missing keys fall back to defaults; unknown keys and wrong types are errors.
use serde::Deserialize;

#[derive(Debug, Default, Deserialize)]
#[serde(default, deny_unknown_fields)]
struct FileConfig {
    server: ServerSection,
    database: DatabaseSection,
    logging: LoggingSection,
}

#[derive(Debug, Deserialize)]
#[serde(default, deny_unknown_fields)]
struct ServerSection {
    addr: String,
    request_timeout_ms: u64,
}

impl Default for ServerSection {
    fn default() -> Self {
        Self {
            addr: "127.0.0.1:8080".into(),
            request_timeout_ms: 5_000,
        }
    }
}

#[derive(Debug, Deserialize)]
#[serde(default, deny_unknown_fields)]
struct DatabaseSection {
    url: String,
    max_connections: u32,
}

impl Default for DatabaseSection {
    fn default() -> Self {
        Self {
            url: "sqlite::memory:".into(),
            max_connections: 1,
        }
    }
}

#[derive(Debug, Default, Deserialize)]
#[serde(default, deny_unknown_fields)]
struct LoggingSection {
    level: Level,
    format: Format,
}

#[derive(Debug, Default, Clone, Copy, Deserialize)]
#[serde(rename_all = "lowercase")]
enum Level {
    Debug,
    #[default]
    Info,
    Warn,
    Error,
}

#[derive(Debug, Default, Clone, Copy, Deserialize)]
#[serde(rename_all = "lowercase")]
enum Format {
    #[default]
    Text,
    Json,
}

const FULL: &str = r#"
[server]
addr = "0.0.0.0:9000"
request_timeout_ms = 2500

[database]
url = "sqlite://tasks.db"
max_connections = 5

[logging]
level = "debug"
format = "json"
"#;

fn summary(config: &FileConfig) -> String {
    format!(
        "addr={} timeout={}ms db={} pool={} log={:?}/{:?}",
        config.server.addr,
        config.server.request_timeout_ms,
        config.database.url,
        config.database.max_connections,
        config.logging.level,
        config.logging.format
    )
}

fn main() {
    println!(
        "full:    {}",
        summary(&toml::from_str(FULL).expect("valid file"))
    );
    println!(
        "partial: {}",
        summary(&toml::from_str("[server]\naddr = \"0.0.0.0:1\"").expect("valid file"))
    );
    println!(
        "empty:   {}",
        summary(&toml::from_str("").expect("valid file"))
    );

    for (name, text) in [
        ("typo", "[server]\nadr = \"0.0.0.0:1\"\n"),
        ("type", "[database]\nmax_connections = \"many\"\n"),
        ("level", "[logging]\nlevel = \"verbose\"\n"),
    ] {
        let error = toml::from_str::<FileConfig>(text).unwrap_err();
        println!("--- {name} error:\n{error}");
    }

    // File errors (missing file) are a different failure from parse errors: report both clearly.
    match std::fs::read_to_string("does-not-exist.toml") {
        Ok(_) => println!("unexpected"),
        Err(error) => println!("read error kind: {:?}", error.kind()),
    }
}
