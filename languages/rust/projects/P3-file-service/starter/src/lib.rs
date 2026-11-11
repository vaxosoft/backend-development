//! P3: a file upload/download service that streams to and from disk.

pub mod error;
pub mod handlers;
pub mod rules;
pub mod store;

use std::io;
use std::path::PathBuf;

use axum::Router;
use axum::extract::DefaultBodyLimit;
use axum::routing::{get, post};

use crate::store::Store;

/// Default upload limit: 5 MiB.
pub const DEFAULT_MAX_SIZE: u64 = 5 * 1024 * 1024;

/// Room for multipart boundaries and headers on top of the file itself.
const MULTIPART_OVERHEAD: u64 = 64 * 1024;

#[derive(Debug, Clone)]
pub struct Config {
    /// Directory for stored files (created if missing).
    pub storage_dir: PathBuf,
    /// Largest accepted file in bytes.
    pub max_size: u64,
}

impl Default for Config {
    fn default() -> Self {
        Self {
            storage_dir: PathBuf::from("storage"),
            max_size: DEFAULT_MAX_SIZE,
        }
    }
}

#[derive(Debug, Clone)]
pub struct AppState {
    pub store: Store,
    pub max_size: u64,
}

/// Opens the storage directory and builds the router.
pub fn build_app(config: &Config) -> io::Result<Router> {
    let state = AppState {
        store: Store::open(&config.storage_dir)?,
        max_size: config.max_size,
    };
    // axum's default body limit is 2 MiB, which would reject legitimate uploads
    // before our own check runs. Raise it to "limit + overhead" as a backstop.
    let upload_limit = usize::try_from(config.max_size + MULTIPART_OVERHEAD).unwrap_or(usize::MAX);
    Ok(Router::new()
        .route(
            "/files",
            post(handlers::upload).layer(DefaultBodyLimit::max(upload_limit)),
        )
        .route(
            "/files/{id}",
            get(handlers::download).delete(handlers::delete),
        )
        .with_state(state))
}
