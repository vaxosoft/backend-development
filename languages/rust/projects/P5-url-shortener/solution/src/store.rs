//! SQL queries. Always bind values with `?`; never format them into the SQL text.

use sqlx::SqlitePool;

use crate::error::ApiError;
use crate::models::{LinkStats, User};

fn is_unique_violation(err: &sqlx::Error) -> bool {
    err.as_database_error()
        .is_some_and(|db| db.is_unique_violation())
}

pub async fn create_user(
    pool: &SqlitePool,
    email: &str,
    password_hash: &str,
) -> Result<i64, ApiError> {
    sqlx::query("INSERT INTO users (email, password_hash) VALUES (?, ?)")
        .bind(email)
        .bind(password_hash)
        .execute(pool)
        .await
        .map(|done| done.last_insert_rowid())
        .map_err(|err| {
            if is_unique_violation(&err) {
                ApiError::Conflict("email is already registered".into())
            } else {
                err.into()
            }
        })
}

pub async fn find_user(pool: &SqlitePool, email: &str) -> Result<Option<User>, ApiError> {
    Ok(
        sqlx::query_as("SELECT id, email, password_hash FROM users WHERE email = ?")
            .bind(email)
            .fetch_optional(pool)
            .await?,
    )
}

pub async fn insert_link(
    pool: &SqlitePool,
    slug: &str,
    url: &str,
    owner_id: i64,
) -> Result<(), ApiError> {
    sqlx::query("INSERT INTO links (slug, url, owner_id) VALUES (?, ?, ?)")
        .bind(slug)
        .bind(url)
        .bind(owner_id)
        .execute(pool)
        .await
        .map(|_| ())
        .map_err(|err| {
            if is_unique_violation(&err) {
                ApiError::Conflict("slug is already taken".into())
            } else {
                err.into()
            }
        })
}

pub async fn record_hit(pool: &SqlitePool, slug: &str) -> Result<Option<String>, ApiError> {
    Ok(
        sqlx::query_scalar("UPDATE links SET hits = hits + 1 WHERE slug = ? RETURNING url")
            .bind(slug)
            .fetch_optional(pool)
            .await?,
    )
}

pub async fn links_of(pool: &SqlitePool, owner_id: i64) -> Result<Vec<LinkStats>, ApiError> {
    Ok(
        sqlx::query_as("SELECT slug, url, hits FROM links WHERE owner_id = ? ORDER BY id")
            .bind(owner_id)
            .fetch_all(pool)
            .await?,
    )
}

pub async fn link_owner(pool: &SqlitePool, slug: &str) -> Result<Option<i64>, ApiError> {
    Ok(
        sqlx::query_scalar("SELECT owner_id FROM links WHERE slug = ?")
            .bind(slug)
            .fetch_optional(pool)
            .await?,
    )
}

pub async fn delete_link(pool: &SqlitePool, slug: &str) -> Result<(), ApiError> {
    sqlx::query("DELETE FROM links WHERE slug = ?")
        .bind(slug)
        .execute(pool)
        .await?;
    Ok(())
}
