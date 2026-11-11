//! Mapping rows to types: `FromRow`, enums stored as TEXT, NULL as `Option`, dates, and decode errors.

use chrono::{DateTime, NaiveDate, TimeZone, Utc};
use sqlx::sqlite::SqlitePoolOptions;
use sqlx::{FromRow, SqlitePool};

#[derive(Debug, Clone, Copy, PartialEq, Eq, sqlx::Type)]
#[sqlx(type_name = "TEXT", rename_all = "snake_case")]
enum Status {
    Todo,
    InProgress,
    Done,
}

#[derive(Debug, FromRow)]
#[allow(dead_code)] // the fields are only displayed through `Debug`
struct Task {
    id: i64,
    title: String,
    status: Status,
    due_date: Option<NaiveDate>,
    owner_id: Option<i64>,
    created_at: DateTime<Utc>,
}

async fn seed(pool: &SqlitePool) -> Result<(), sqlx::Error> {
    sqlx::query(
        "CREATE TABLE tasks (
            id INTEGER PRIMARY KEY, title TEXT NOT NULL, status TEXT NOT NULL,
            due_date TEXT, owner_id INTEGER, created_at TEXT NOT NULL)",
    )
    .execute(pool)
    .await?;
    let created_at = Utc.with_ymd_and_hms(2030, 1, 1, 9, 0, 0).unwrap();
    sqlx::query(
        "INSERT INTO tasks (title, status, due_date, owner_id, created_at) VALUES (?, ?, ?, ?, ?)",
    )
    .bind("Write docs")
    .bind(Status::InProgress)
    .bind(NaiveDate::from_ymd_opt(2030, 1, 31))
    .bind(7_i64)
    .bind(created_at)
    .execute(pool)
    .await?;
    sqlx::query("INSERT INTO tasks (title, status, created_at) VALUES (?, ?, ?)")
        .bind("Review PR")
        .bind(Status::Todo)
        .bind(created_at)
        .execute(pool)
        .await?;
    Ok(())
}

#[tokio::main]
async fn main() -> Result<(), sqlx::Error> {
    let pool = SqlitePoolOptions::new()
        .max_connections(1)
        .connect("sqlite::memory:")
        .await?;
    seed(&pool).await?;

    // Columns are matched to fields by name; the order in SELECT does not matter.
    let tasks: Vec<Task> = sqlx::query_as("SELECT * FROM tasks ORDER BY id")
        .fetch_all(&pool)
        .await?;
    for task in &tasks {
        println!("{task:?}");
    }

    // What is actually stored: enums and timestamps are plain TEXT in SQLite.
    let raw: (String, String) = sqlx::query_as("SELECT status, created_at FROM tasks WHERE id = 1")
        .fetch_one(&pool)
        .await?;
    println!("stored as {raw:?}");

    // Filtering on an enum binds it exactly like any other value.
    let todo: Vec<Task> = sqlx::query_as("SELECT * FROM tasks WHERE status = ?")
        .bind(Status::Todo)
        .fetch_all(&pool)
        .await?;
    println!(
        "todo: {:?}",
        todo.iter().map(|t| t.title.as_str()).collect::<Vec<_>>()
    );

    // Decode failures are runtime errors (query_as is checked when it runs, not when it compiles).
    let err = sqlx::query_as::<_, Task>("SELECT id, title FROM tasks")
        .fetch_all(&pool)
        .await
        .unwrap_err();
    println!("missing column -> {err}");

    sqlx::query("UPDATE tasks SET status = 'archived' WHERE id = 2")
        .execute(&pool)
        .await?;
    let err = sqlx::query_as::<_, Task>("SELECT * FROM tasks WHERE id = 2")
        .fetch_one(&pool)
        .await
        .unwrap_err();
    println!("unknown status -> {err}");
    Ok(())
}
