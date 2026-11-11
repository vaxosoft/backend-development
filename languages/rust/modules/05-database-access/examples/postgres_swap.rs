//! PostgreSQL is the same API with a different driver: `PgPool`, `$1` placeholders, `DATABASE_URL`.
//!
//! Compile-checked always; it only connects when `DATABASE_URL` is set, e.g.
//! `DATABASE_URL=postgres://user:pass@localhost/tasks cargo run -p m05-database-access --example postgres_swap`

use sqlx::postgres::PgPoolOptions;

#[tokio::main]
async fn main() -> Result<(), sqlx::Error> {
    let Ok(url) = std::env::var("DATABASE_URL") else {
        println!("DATABASE_URL is not set: skipping the PostgreSQL demo");
        return Ok(());
    };

    // A real server handles many connections: size the pool for it.
    let pool = PgPoolOptions::new()
        .max_connections(5)
        .connect(&url)
        .await?;

    // Isolation level is chosen per transaction, as the first statement after BEGIN.
    let mut tx = pool.begin().await?;
    sqlx::query("SET TRANSACTION ISOLATION LEVEL REPEATABLE READ")
        .execute(&mut *tx)
        .await?;

    // TEMP tables vanish with the connection, so the demo leaves nothing behind.
    sqlx::query(
        "CREATE TEMP TABLE tasks_demo (
            id       BIGSERIAL PRIMARY KEY,
            title    TEXT NOT NULL,
            due_date DATE
        )",
    )
    .execute(&mut *tx)
    .await?;

    // Differences from SQLite: `$1, $2` placeholders and native DATE/TIMESTAMPTZ types.
    let id: i64 =
        sqlx::query_scalar("INSERT INTO tasks_demo (title, due_date) VALUES ($1, $2) RETURNING id")
            .bind("Write docs")
            .bind(chrono::NaiveDate::from_ymd_opt(2030, 1, 31))
            .fetch_one(&mut *tx)
            .await?;
    println!("inserted task {id}");

    tx.rollback().await?;
    pool.close().await;
    Ok(())
}
