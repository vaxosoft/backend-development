//! SQLite connection and schema (provided).

use sqlx::SqlitePool;
use sqlx::sqlite::{SqliteConnectOptions, SqlitePoolOptions};

const SCHEMA: &[&str] = &[
    "CREATE TABLE IF NOT EXISTS users (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        email TEXT NOT NULL UNIQUE,
        password_hash TEXT NOT NULL
    )",
    "CREATE TABLE IF NOT EXISTS links (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        slug TEXT NOT NULL UNIQUE,
        url TEXT NOT NULL,
        owner_id INTEGER NOT NULL REFERENCES users(id),
        hits INTEGER NOT NULL DEFAULT 0
    )",
];

/// `sqlite::memory:` gives every connection its own empty database, so a single
/// connection keeps the schema and the data in one place.
pub async fn connect(url: &str) -> Result<SqlitePool, sqlx::Error> {
    let options: SqliteConnectOptions =
        url.parse::<SqliteConnectOptions>()?.create_if_missing(true);
    let pool = SqlitePoolOptions::new()
        .max_connections(1)
        .connect_with(options)
        .await?;
    for statement in SCHEMA {
        sqlx::query(*statement).execute(&pool).await?;
    }
    Ok(pool)
}
