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
    let _ = (pool, name, email);
    todo!("INSERT ... RETURNING id, name, email, mapped with query_as")
}

/// `Ok(None)` when no user has that email.
pub async fn find_by_email(pool: &SqlitePool, email: &str) -> Result<Option<User>, sqlx::Error> {
    let _ = (pool, email);
    todo!("query_as + fetch_optional")
}

/// All users, lowest id first.
pub async fn list_users(pool: &SqlitePool) -> Result<Vec<User>, sqlx::Error> {
    let _ = pool;
    todo!("query_as + fetch_all")
}

pub async fn count_users(pool: &SqlitePool) -> Result<i64, sqlx::Error> {
    let _ = pool;
    todo!("query_scalar + fetch_one")
}
