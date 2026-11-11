//! The only module that talks to SQLite. Handlers never see SQL.

use std::str::FromStr;

use chrono::{DateTime, NaiveDate, SubsecRound, Utc};
use sqlx::sqlite::{SqliteConnectOptions, SqlitePoolOptions};
use sqlx::{AssertSqlSafe, FromRow, SqlitePool};

use crate::model::{ListParams, NewTask, Page, Sort, Status, Task};

const SCHEMA: &str = "
CREATE TABLE IF NOT EXISTS tasks (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    title      TEXT    NOT NULL,
    status     TEXT    NOT NULL,
    due_date   TEXT,
    tags       TEXT    NOT NULL,   -- JSON array, e.g. [\"docs\",\"p1\"]
    owner_id   INTEGER,
    created_at TEXT    NOT NULL
)";

#[derive(Clone)]
pub struct TaskRepo {
    pool: SqlitePool,
}

/// A row as SQLite stores it; `into_task` converts it to the domain type.
#[derive(Debug, FromRow)]
pub struct TaskRow {
    id: i64,
    title: String,
    status: String,
    due_date: Option<NaiveDate>,
    tags: String,
    owner_id: Option<i64>,
    created_at: DateTime<Utc>,
}

impl TaskRow {
    pub fn into_task(self) -> Result<Task, sqlx::Error> {
        let decode = |msg: String| sqlx::Error::Decode(msg.into());
        Ok(Task {
            id: self.id,
            title: self.title,
            status: Status::parse(&self.status)
                .ok_or_else(|| decode(format!("unknown status {:?}", self.status)))?,
            due_date: self.due_date,
            tags: serde_json::from_str(&self.tags).map_err(|e| decode(e.to_string()))?,
            owner_id: self.owner_id,
            created_at: self.created_at,
        })
    }
}

/// Tags are stored as a JSON array in one TEXT column.
pub fn tags_to_json(tags: &[String]) -> String {
    serde_json::to_string(tags).expect("a list of strings always serializes")
}

impl TaskRepo {
    /// Opens the database and creates the schema.
    ///
    /// One connection only: every connection to `sqlite::memory:` would be a
    /// separate empty database, and SQLite allows a single writer anyway.
    pub async fn connect(database_url: &str) -> Result<Self, sqlx::Error> {
        let options = SqliteConnectOptions::from_str(database_url)?.create_if_missing(true);
        let pool = SqlitePoolOptions::new()
            .max_connections(1)
            .idle_timeout(None)
            .max_lifetime(None)
            .connect_with(options)
            .await?;
        sqlx::query(SCHEMA).execute(&pool).await?;
        Ok(Self { pool })
    }

    pub async fn create(&self, new: &NewTask) -> Result<Task, sqlx::Error> {
        let row = sqlx::query_as::<_, TaskRow>(
            "INSERT INTO tasks (title, status, due_date, tags, owner_id, created_at)
             VALUES (?, ?, ?, ?, ?, ?) RETURNING *",
        )
        .bind(&new.title)
        .bind(new.status.as_str())
        .bind(new.due_date)
        .bind(tags_to_json(&new.tags))
        .bind(new.owner_id)
        .bind(Utc::now().trunc_subsecs(0))
        .fetch_one(&self.pool)
        .await?;
        row.into_task()
    }

    pub async fn get(&self, id: i64) -> Result<Option<Task>, sqlx::Error> {
        let row = sqlx::query_as::<_, TaskRow>("SELECT * FROM tasks WHERE id = ?")
            .bind(id)
            .fetch_optional(&self.pool)
            .await?;
        row.map(TaskRow::into_task).transpose()
    }

    pub async fn replace(&self, id: i64, new: &NewTask) -> Result<Option<Task>, sqlx::Error> {
        let row = sqlx::query_as::<_, TaskRow>(
            "UPDATE tasks SET title = ?, status = ?, due_date = ?, tags = ?, owner_id = ?
             WHERE id = ? RETURNING *",
        )
        .bind(&new.title)
        .bind(new.status.as_str())
        .bind(new.due_date)
        .bind(tags_to_json(&new.tags))
        .bind(new.owner_id)
        .bind(id)
        .fetch_optional(&self.pool)
        .await?;
        row.map(TaskRow::into_task).transpose()
    }

    pub async fn delete(&self, id: i64) -> Result<bool, sqlx::Error> {
        let result = sqlx::query("DELETE FROM tasks WHERE id = ?")
            .bind(id)
            .execute(&self.pool)
            .await?;
        Ok(result.rows_affected() > 0)
    }

    pub async fn list(&self, params: &ListParams) -> Result<Page, sqlx::Error> {
        // ?1 = status, ?2 = tag; a NULL parameter disables that filter.
        const FILTER: &str = "WHERE (?1 IS NULL OR status = ?1)
            AND (?2 IS NULL OR EXISTS (
                SELECT 1 FROM json_each(tasks.tags) WHERE json_each.value = ?2))";
        // sqlx 0.9 only takes `&'static str` unless the SQL is wrapped in
        // `AssertSqlSafe`. That is fine here: every fragment below is a
        // constant of ours, never user input (user values are bound).
        let order_by = match params.sort {
            Sort::CreatedAsc => "created_at ASC, id ASC",
            Sort::CreatedDesc => "created_at DESC, id DESC",
            Sort::DueDateAsc => "due_date IS NULL, due_date ASC, id ASC",
            Sort::DueDateDesc => "due_date IS NULL, due_date DESC, id DESC",
        };
        let status = params.status.map(Status::as_str);

        let count_sql = format!("SELECT COUNT(*) FROM tasks {FILTER}");
        let total: i64 = sqlx::query_scalar(AssertSqlSafe(count_sql))
            .bind(status)
            .bind(params.tag.as_deref())
            .fetch_one(&self.pool)
            .await?;

        let page_sql =
            format!("SELECT * FROM tasks {FILTER} ORDER BY {order_by} LIMIT ?3 OFFSET ?4");
        let rows = sqlx::query_as::<_, TaskRow>(AssertSqlSafe(page_sql))
            .bind(status)
            .bind(params.tag.as_deref())
            .bind(params.limit)
            .bind(params.offset)
            .fetch_all(&self.pool)
            .await?;
        let items = rows
            .into_iter()
            .map(TaskRow::into_task)
            .collect::<Result<_, _>>()?;
        Ok(Page { items, total })
    }
}
