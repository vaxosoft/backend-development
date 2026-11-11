//! Connect, execute, bind parameters, read rows, handle NULL and "no rows".

use sqlx::Row;
use sqlx::sqlite::SqlitePoolOptions;

#[tokio::main]
async fn main() -> Result<(), sqlx::Error> {
    // In-memory database: it lives only as long as its connections, so keep exactly one.
    let pool = SqlitePoolOptions::new()
        .max_connections(1)
        .connect("sqlite::memory:")
        .await?;

    sqlx::query("CREATE TABLE tasks (id INTEGER PRIMARY KEY, title TEXT NOT NULL, due_date TEXT)")
        .execute(&pool)
        .await?;

    // `?` placeholders + bind: values travel separately from the SQL text.
    let done = sqlx::query("INSERT INTO tasks (title, due_date) VALUES (?, ?)")
        .bind("Write docs")
        .bind("2030-01-31")
        .execute(&pool)
        .await?;
    println!(
        "rows_affected={} last_insert_rowid={}",
        done.rows_affected(),
        done.last_insert_rowid()
    );

    // `Option<T>` binds as NULL when it is `None`.
    let due: Option<&str> = None;
    sqlx::query("INSERT INTO tasks (title, due_date) VALUES (?, ?)")
        .bind("Review PR")
        .bind(due)
        .execute(&pool)
        .await?;

    // A hostile title is just data.
    let hostile = "x'); DROP TABLE tasks; --";
    sqlx::query("INSERT INTO tasks (title) VALUES (?)")
        .bind(hostile)
        .execute(&pool)
        .await?;

    // Rows by column name; NULL needs `Option` on the Rust side.
    let rows = sqlx::query("SELECT id, title, due_date FROM tasks ORDER BY id")
        .fetch_all(&pool)
        .await?;
    for row in &rows {
        let id: i64 = row.get("id");
        let title: String = row.get("title");
        let due: Option<String> = row.get("due_date");
        println!("{id} | {title} | {due:?}");
    }

    // One value: query_scalar. Optional row: fetch_optional. Required row: fetch_one.
    let count: i64 = sqlx::query_scalar("SELECT COUNT(*) FROM tasks")
        .fetch_one(&pool)
        .await?;
    println!("count={count}");

    let missing = sqlx::query("SELECT title FROM tasks WHERE id = ?")
        .bind(99)
        .fetch_optional(&pool)
        .await?;
    println!("fetch_optional -> is_none={}", missing.is_none());

    let err = sqlx::query("SELECT title FROM tasks WHERE id = ?")
        .bind(99)
        .fetch_one(&pool)
        .await
        .unwrap_err();
    println!("fetch_one -> {err}");

    // Each `connect("sqlite::memory:")` creates its own private database, so a second pool
    // (say, built by a helper that is called twice) starts empty.
    let other = SqlitePoolOptions::new()
        .max_connections(1)
        .connect("sqlite::memory:")
        .await?;
    let err = sqlx::query("SELECT COUNT(*) FROM tasks")
        .fetch_one(&other)
        .await
        .unwrap_err();
    println!("another pool -> {err}");
    Ok(())
}
