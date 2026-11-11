//! P5: a URL shortener with accounts, JWT auth, a per-user rate limit and JSON request logs.

pub mod auth;
pub mod db;
pub mod error;
pub mod handlers;
pub mod jwt_crypto;
pub mod limiter;
pub mod logging;
pub mod models;
pub mod store;

use std::sync::Arc;

use axum::Router;
use axum::middleware::from_fn_with_state;
use axum::routing::{delete, get, post};
use sqlx::SqlitePool;

use crate::error::ApiError;
use crate::limiter::RateLimiter;

/// Receives one JSON log line per request (without a trailing newline).
pub type LogSink = Arc<dyn Fn(String) + Send + Sync>;

#[derive(Clone)]
pub struct Config {
    pub database_url: String,
    pub jwt_secret: String,
    /// Token lifetime in seconds. Negative values mint already-expired tokens (useful in tests).
    pub token_ttl_secs: i64,
    pub rate_burst: u32,
    pub rate_refill_per_sec: f64,
    /// Argon2 memory cost in KiB. Production default is OWASP's 19 MiB; tests lower it for speed.
    pub hash_memory_kib: u32,
    pub log_sink: LogSink,
}

impl Config {
    pub fn new(jwt_secret: impl Into<String>) -> Self {
        Self {
            database_url: "sqlite::memory:".into(),
            jwt_secret: jwt_secret.into(),
            token_ttl_secs: 3600,
            rate_burst: 5,
            rate_refill_per_sec: 1.0,
            hash_memory_kib: 19 * 1024,
            log_sink: Arc::new(|line| println!("{line}")),
        }
    }
}

#[derive(Clone)]
pub struct AppState {
    pub pool: SqlitePool,
    pub config: Arc<Config>,
    pub limiter: Arc<RateLimiter>,
}

/// Opens the database, creates the schema and wires the routes and middleware.
pub async fn build_app(config: Config) -> Result<Router, sqlx::Error> {
    jwt_crypto::install_jwt_crypto();
    let state = AppState {
        pool: db::connect(&config.database_url).await?,
        limiter: Arc::new(RateLimiter::new(
            config.rate_burst,
            config.rate_refill_per_sec,
        )),
        config: Arc::new(config),
    };
    Ok(Router::new()
        .route("/auth/register", post(handlers::register))
        .route("/auth/login", post(handlers::login))
        .route(
            "/links",
            post(handlers::create_link).get(handlers::list_links),
        )
        .route("/links/{slug}", delete(handlers::delete_link))
        .route("/{slug}", get(handlers::redirect))
        .fallback(|| async { ApiError::NotFound })
        .layer(from_fn_with_state(state.clone(), logging::log_requests))
        .with_state(state))
}
