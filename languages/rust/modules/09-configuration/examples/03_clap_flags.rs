//! Command-line flags with clap's derive API. `env = ...` makes a flag fall back to a variable.
//! Precedence inside clap: command line > environment variable > default.
use std::net::SocketAddr;
use std::path::PathBuf;

use clap::{Parser, ValueEnum};

#[derive(Debug, Clone, Copy, PartialEq, ValueEnum)]
enum Level {
    Debug,
    Info,
    Warn,
}

#[derive(Debug, Parser)]
#[command(name = "tasktracker", version, about = "Task Tracker server")]
struct Cli {
    /// Address to listen on
    #[arg(long, env = "TT_ADDR", default_value = "127.0.0.1:8080")]
    addr: SocketAddr,

    #[arg(long, env = "TT_LOG_LEVEL", value_enum, default_value_t = Level::Info)]
    log_level: Level,

    /// Per-request timeout in milliseconds
    #[arg(long, env = "TT_TIMEOUT_MS", default_value_t = 5_000, value_parser = clap::value_parser!(u64).range(1..=60_000))]
    timeout_ms: u64,

    /// Optional TOML config file
    #[arg(long, env = "TT_CONFIG")]
    config: Option<PathBuf>,

    /// Repeat for more verbosity (-v, -vv)
    #[arg(short, long, action = clap::ArgAction::Count)]
    verbose: u8,
}

fn main() {
    // `try_parse_from` returns the error instead of printing and exiting: easy to demo and test.
    // A real `main` calls `Cli::parse()`, which prints errors/`--help` and exits.
    let runs: [&[&str]; 6] = [
        &["tasktracker"],
        &[
            "tasktracker",
            "--addr",
            "0.0.0.0:9000",
            "--log-level",
            "debug",
            "-vv",
        ],
        &[
            "tasktracker",
            "--addr=[::1]:3000",
            "--config",
            "tasks.toml",
            "--timeout-ms",
            "250",
        ],
        &["tasktracker", "--addr", "localhost"],
        &["tasktracker", "--timeout-ms", "0"],
        &["tasktracker", "--log-level", "loud"],
    ];
    for args in runs {
        println!("$ {}", args.join(" "));
        match Cli::try_parse_from(args) {
            Ok(cli) => println!(
                "  addr={} level={:?} timeout={}ms config={:?} verbose={}",
                cli.addr, cli.log_level, cli.timeout_ms, cli.config, cli.verbose
            ),
            Err(error) => println!("  {}", error.to_string().lines().next().unwrap_or_default()),
        }
    }
}
