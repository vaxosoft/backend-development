//! Exercise 1: bind parameters, map rows to a struct, handle "no row" and constraint errors.
//!
//! `migrate` and `memory_pool` are done for you. Implement the four queries with
//! `sqlx::query_as` / `sqlx::query_scalar` and `.bind(...)` (never `format!` values
//! into SQL).

use sqlx::sqlite::SqlitePoolOptions;
use sqlx::{FromRow, SqlitePool};

#[derive(Debug, Clone, PartialEq, FromRow)]
pub struct User {
    pub id: i64,
    pub name: String,
    pub email: String,
}

/// One connection: the in-memory database lives only as long as its connections.
pub async fn memory_pool() -> SqlitePool {
    SqlitePoolOptions::new()
        .max_connections(1)
        .connect("sqlite::memory:")
        .await
        .expect("open in-memory SQLite")
}

pub async fn migrate(pool: &SqlitePool) -> Result<(), sqlx::Error> {
    sqlx::query(
        "CREATE TABLE users (
            id    INTEGER PRIMARY KEY AUTOINCREMENT,
            name  TEXT NOT NULL,
            email TEXT NOT NULL UNIQUE
        )",
    )
    .execute(pool)
    .await?;
    Ok(())
}

/// Inserts a user and returns it with its database-assigned id.
/// A duplicate email must surface as the driver's unique-violation error.
pub async fn insert_user(pool: &SqlitePool, name: &str, email: &str) -> Result<User, sqlx::Error> {
    sqlx::query_as("INSERT INTO users (name, email) VALUES (?, ?) RETURNING id, name, email")
        .bind(name)
        .bind(email)
        .fetch_one(pool)
        .await
}

/// `Ok(None)` when no user has that email.
pub async fn find_by_email(pool: &SqlitePool, email: &str) -> Result<Option<User>, sqlx::Error> {
    sqlx::query_as("SELECT id, name, email FROM users WHERE email = ?")
        .bind(email)
        .fetch_optional(pool)
        .await
}

/// All users, lowest id first.
pub async fn list_users(pool: &SqlitePool) -> Result<Vec<User>, sqlx::Error> {
    sqlx::query_as("SELECT id, name, email FROM users ORDER BY id")
        .fetch_all(pool)
        .await
}

pub async fn count_users(pool: &SqlitePool) -> Result<i64, sqlx::Error> {
    sqlx::query_scalar("SELECT COUNT(*) FROM users")
        .fetch_one(pool)
        .await
}
