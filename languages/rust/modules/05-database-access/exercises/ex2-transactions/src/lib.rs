//! Exercise 2: transactions - all or nothing.
//!
//! Both functions touch two tables and must be atomic: if anything fails half way,
//! the database must look as if the call never happened. Use `pool.begin()`,
//! run every statement on `&mut *tx`, and `commit()` only at the end. Returning
//! early with `?` drops the transaction, which rolls it back.

use sqlx::SqlitePool;
use sqlx::sqlite::SqlitePoolOptions;

pub async fn memory_pool() -> SqlitePool {
    SqlitePoolOptions::new()
        .max_connections(1)
        .connect("sqlite::memory:")
        .await
        .expect("open in-memory SQLite")
}

pub async fn migrate(pool: &SqlitePool) -> Result<(), sqlx::Error> {
    sqlx::query("CREATE TABLE tasks (id INTEGER PRIMARY KEY AUTOINCREMENT, title TEXT NOT NULL)")
        .execute(pool)
        .await?;
    sqlx::query(
        "CREATE TABLE task_tags (
            task_id INTEGER NOT NULL REFERENCES tasks(id),
            tag     TEXT NOT NULL,
            PRIMARY KEY (task_id, tag)
        )",
    )
    .execute(pool)
    .await?;
    Ok(())
}

/// Inserts the task and one `task_tags` row per tag, atomically.
/// A repeated tag violates the primary key: the task row must not survive.
pub async fn create_task_with_tags(
    pool: &SqlitePool,
    title: &str,
    tags: &[&str],
) -> Result<i64, sqlx::Error> {
    let _ = (pool, title, tags);
    todo!("begin, insert task, insert tags on the transaction, commit")
}

/// Replaces all tags of a task atomically. If the new list is invalid
/// (for example contains duplicates) the old tags must still be there afterwards.
pub async fn replace_tags(
    pool: &SqlitePool,
    task_id: i64,
    tags: &[&str],
) -> Result<(), sqlx::Error> {
    let _ = (pool, task_id, tags);
    todo!("begin, delete old tags, insert new ones, commit")
}

pub async fn count_tasks(pool: &SqlitePool) -> Result<i64, sqlx::Error> {
    sqlx::query_scalar("SELECT COUNT(*) FROM tasks")
        .fetch_one(pool)
        .await
}

pub async fn tags_of(pool: &SqlitePool, task_id: i64) -> Result<Vec<String>, sqlx::Error> {
    sqlx::query_scalar("SELECT tag FROM task_tags WHERE task_id = ? ORDER BY tag")
        .bind(task_id)
        .fetch_all(pool)
        .await
}
