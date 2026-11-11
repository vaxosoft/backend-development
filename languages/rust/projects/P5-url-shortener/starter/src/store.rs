//! SQL queries. Always bind values with `?`; never format them into the SQL text.

use sqlx::SqlitePool;

use crate::error::ApiError;
use crate::models::{LinkStats, User};

/// TODO: insert a user and return its id. A duplicate email is `ApiError::Conflict`.
pub async fn create_user(
    _pool: &SqlitePool,
    _email: &str,
    _password_hash: &str,
) -> Result<i64, ApiError> {
    todo!("INSERT INTO users")
}

/// TODO: look a user up by (already normalised) email.
pub async fn find_user(_pool: &SqlitePool, _email: &str) -> Result<Option<User>, ApiError> {
    todo!("SELECT FROM users")
}

/// TODO: insert a link. A duplicate slug is `ApiError::Conflict`.
pub async fn insert_link(
    _pool: &SqlitePool,
    _slug: &str,
    _url: &str,
    _owner_id: i64,
) -> Result<(), ApiError> {
    todo!("INSERT INTO links")
}

/// TODO: count a hit and return the target URL, or `None` for an unknown slug.
/// Do it in ONE statement (`UPDATE ... RETURNING`) so concurrent hits are not lost.
pub async fn record_hit(_pool: &SqlitePool, _slug: &str) -> Result<Option<String>, ApiError> {
    todo!("UPDATE links SET hits = hits + 1")
}

/// TODO: the owner's links, oldest first.
pub async fn links_of(_pool: &SqlitePool, _owner_id: i64) -> Result<Vec<LinkStats>, ApiError> {
    todo!("SELECT FROM links")
}

/// TODO: the owner id of a slug, or `None` when it does not exist.
pub async fn link_owner(_pool: &SqlitePool, _slug: &str) -> Result<Option<i64>, ApiError> {
    todo!("SELECT owner_id")
}

/// TODO: delete a link by slug.
pub async fn delete_link(_pool: &SqlitePool, _slug: &str) -> Result<(), ApiError> {
    todo!("DELETE FROM links")
}
