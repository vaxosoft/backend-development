//! Integration tests against a real (in-memory) SQLite database, one fresh database per test.

use chrono::{DateTime, TimeZone, Utc};
use sqlx::SqlitePool;
use sqlx::sqlite::SqlitePoolOptions;

struct SqliteTasks {
    pool: SqlitePool,
}

impl SqliteTasks {
    async fn new() -> Self {
        // Every connection to `sqlite::memory:` is its own database, so the pool must hold exactly one
        // and must never recycle it (idle timeout / max lifetime would silently drop the data).
        let pool = SqlitePoolOptions::new()
            .max_connections(1)
            .idle_timeout(None)
            .max_lifetime(None)
            .connect("sqlite::memory:")
            .await
            .expect("open in-memory database");
        sqlx::query(
            "CREATE TABLE tasks (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                title TEXT NOT NULL CHECK (length(title) BETWEEN 1 AND 200),
                status TEXT NOT NULL DEFAULT 'todo' CHECK (status IN ('todo', 'in_progress', 'done')),
                created_at TEXT NOT NULL
            )",
        )
        .execute(&pool)
        .await
        .expect("create schema");
        Self { pool }
    }

    async fn insert(&self, title: &str, created_at: DateTime<Utc>) -> Result<i64, sqlx::Error> {
        let row: (i64,) =
            sqlx::query_as("INSERT INTO tasks (title, created_at) VALUES (?, ?) RETURNING id")
                .bind(title)
                .bind(created_at)
                .fetch_one(&self.pool)
                .await?;
        Ok(row.0)
    }

    async fn titles(&self) -> Vec<String> {
        sqlx::query_scalar("SELECT title FROM tasks ORDER BY id")
            .fetch_all(&self.pool)
            .await
            .expect("select titles")
    }
}

fn t0() -> DateTime<Utc> {
    Utc.with_ymd_and_hms(2030, 1, 1, 9, 0, 0).unwrap()
}

#[tokio::test]
async fn every_test_gets_its_own_database() {
    // These run in parallel with the other tests; ids start at 1 in each because nothing is shared.
    let db = SqliteTasks::new().await;
    assert_eq!(db.insert("first", t0()).await.unwrap(), 1);
    assert_eq!(db.insert("second", t0()).await.unwrap(), 2);
}

#[tokio::test]
async fn schema_constraints_reject_bad_rows() {
    let db = SqliteTasks::new().await;
    assert!(db.insert("", t0()).await.is_err());

    db.insert("ok", t0()).await.unwrap();
    let bad_status = sqlx::query("UPDATE tasks SET status = 'blocked'")
        .execute(&db.pool)
        .await;
    assert!(bad_status.is_err());
}

#[tokio::test]
async fn bound_parameters_neutralise_sql_injection() {
    let db = SqliteTasks::new().await;
    let nasty = "x'); DROP TABLE tasks; --";
    db.insert(nasty, t0()).await.unwrap();
    assert_eq!(db.titles().await, [nasty]);
}

#[tokio::test]
async fn dropping_a_transaction_rolls_it_back() {
    let db = SqliteTasks::new().await;
    db.insert("kept", t0()).await.unwrap();

    {
        let mut tx = db.pool.begin().await.unwrap();
        sqlx::query(
            "INSERT INTO tasks (title, created_at) VALUES ('temporary', '2030-01-01T00:00:00Z')",
        )
        .execute(&mut *tx)
        .await
        .unwrap();
        // no commit: dropping `tx` rolls back
    }
    assert_eq!(db.titles().await, ["kept"]);
}
