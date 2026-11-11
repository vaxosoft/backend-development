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
    let mut tx = pool.begin().await?;
    let id: i64 = sqlx::query_scalar("INSERT INTO tasks (title) VALUES (?) RETURNING id")
        .bind(title)
        .fetch_one(&mut *tx)
        .await?;
    for tag in tags {
        sqlx::query("INSERT INTO task_tags (task_id, tag) VALUES (?, ?)")
            .bind(id)
            .bind(tag)
            .execute(&mut *tx)
            .await?;
    }
    tx.commit().await?;
    Ok(id)
}

/// Replaces all tags of a task atomically. If the new list is invalid
/// (for example contains duplicates) the old tags must still be there afterwards.
pub async fn replace_tags(
    pool: &SqlitePool,
    task_id: i64,
    tags: &[&str],
) -> Result<(), sqlx::Error> {
    let mut tx = pool.begin().await?;
    sqlx::query("DELETE FROM task_tags WHERE task_id = ?")
        .bind(task_id)
        .execute(&mut *tx)
        .await?;
    for tag in tags {
        sqlx::query("INSERT INTO task_tags (task_id, tag) VALUES (?, ?)")
            .bind(task_id)
            .bind(tag)
            .execute(&mut *tx)
            .await?;
    }
    tx.commit().await
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
