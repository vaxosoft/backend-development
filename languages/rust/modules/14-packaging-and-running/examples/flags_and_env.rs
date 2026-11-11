//! Runtime settings: defaults < environment < command-line flags, validated before the server starts.

use std::net::SocketAddr;

use clap::{Parser, ValueEnum};

#[derive(Clone, Copy, Debug, ValueEnum)]
enum LogLevel {
    Debug,
    Info,
    Warn,
    Error,
}

/// Task Tracker service
#[derive(Debug, Parser)]
#[command(name = "tasks-service", version)]
struct Settings {
    /// Address to listen on
    #[arg(long, env = "ADDR", default_value = "127.0.0.1:8080")]
    addr: SocketAddr,

    /// Minimum log level
    #[arg(long, env = "LOG_LEVEL", value_enum, default_value_t = LogLevel::Info)]
    log_level: LogLevel,

    /// Number of async worker threads
    #[arg(long, env = "WORKERS", default_value_t = 4, value_parser = clap::value_parser!(u16).range(1..=256))]
    workers: u16,

    /// Seconds to wait for in-flight requests on shutdown
    #[arg(long, env = "SHUTDOWN_TIMEOUT_SECS", default_value_t = 10)]
    shutdown_timeout_secs: u64,
}

fn main() {
    // On bad input clap prints a message to stderr and exits with status 2.
    let settings = Settings::parse();
    println!("{settings:#?}");
}
