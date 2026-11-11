//! Transactions: commit, rollback on drop, atomic multi-step work, and pool starvation.

use std::time::Duration;

use sqlx::SqlitePool;
use sqlx::sqlite::SqlitePoolOptions;

const TASKS: &str = "SELECT COUNT(*) FROM tasks";
const AUDIT: &str = "SELECT COUNT(*) FROM audit";

/// `sql` must be a `&'static str`: sqlx 0.9 refuses dynamically built SQL unless you opt in.
async fn count(pool: &SqlitePool, sql: &'static str) -> Result<i64, sqlx::Error> {
    sqlx::query_scalar(sql).fetch_one(pool).await
}

/// Both inserts succeed or neither does. Returning early with `?` drops `tx`, which rolls back.
async fn create_audited(pool: &SqlitePool, title: &str, note: &str) -> Result<i64, sqlx::Error> {
    let mut tx = pool.begin().await?;
    let id: i64 = sqlx::query_scalar("INSERT INTO tasks (title) VALUES (?) RETURNING id")
        .bind(title)
        .fetch_one(&mut *tx)
        .await?;
    sqlx::query("INSERT INTO audit (task_id, note) VALUES (?, ?)")
        .bind(id)
        .bind(note)
        .execute(&mut *tx)
        .await?;
    tx.commit().await?;
    Ok(id)
}

#[tokio::main]
async fn main() -> Result<(), sqlx::Error> {
    let pool = SqlitePoolOptions::new()
        .max_connections(1)
        .acquire_timeout(Duration::from_millis(100))
        .connect("sqlite::memory:")
        .await?;
    sqlx::query("CREATE TABLE tasks (id INTEGER PRIMARY KEY, title TEXT NOT NULL)")
        .execute(&pool)
        .await?;
    sqlx::query("CREATE TABLE audit (task_id INTEGER NOT NULL, note TEXT NOT NULL CHECK (length(note) > 0))")
        .execute(&pool)
        .await?;

    // Commit makes the work visible.
    let id = create_audited(&pool, "Write docs", "created").await?;
    println!(
        "committed task {id}: tasks={} audit={}",
        count(&pool, TASKS).await?,
        count(&pool, AUDIT).await?
    );

    // The audit insert violates the CHECK: the task insert before it is rolled back too.
    let err = create_audited(&pool, "Broken", "").await.unwrap_err();
    println!("failed: {err}");
    println!(
        "after rollback: tasks={} audit={}",
        count(&pool, TASKS).await?,
        count(&pool, AUDIT).await?
    );

    // Explicit rollback, and rollback by simply dropping the transaction.
    let mut tx = pool.begin().await?;
    sqlx::query("INSERT INTO tasks (title) VALUES ('temp')")
        .execute(&mut *tx)
        .await?;
    let inside: i64 = sqlx::query_scalar("SELECT COUNT(*) FROM tasks")
        .fetch_one(&mut *tx)
        .await?;
    println!("inside the transaction: tasks={inside}");
    tx.rollback().await?;
    {
        let mut tx = pool.begin().await?;
        sqlx::query("INSERT INTO tasks (title) VALUES ('temp2')")
            .execute(&mut *tx)
            .await?;
    } // dropped without commit
    println!("after rollbacks: tasks={}", count(&pool, TASKS).await?);

    // Pitfall: while a transaction holds the only connection, the pool cannot serve anyone else.
    let tx = pool.begin().await?;
    let blocked = count(&pool, TASKS).await.unwrap_err();
    println!("using `pool` inside a transaction -> {blocked}");
    drop(tx);
    println!("after drop: tasks={}", count(&pool, TASKS).await?);
    Ok(())
}
