//! The only module that talks to SQLite. Handlers never see SQL.

use std::str::FromStr;

use chrono::{DateTime, NaiveDate, Utc};
use sqlx::sqlite::{SqliteConnectOptions, SqlitePoolOptions};
use sqlx::{FromRow, SqlitePool};

use crate::model::{ListParams, NewTask, Page, Status, Task};

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

    /// TODO: insert and return the stored task (`created_at` = now, whole
    /// seconds, UTC). `INSERT ... RETURNING *` avoids a second query and
    /// decodes straight into `TaskRow`.
    pub async fn create(&self, _new: &NewTask) -> Result<Task, sqlx::Error> {
        let _ = &self.pool;
        todo!("TaskRepo::create")
    }

    /// TODO: `Ok(None)` when the id does not exist.
    pub async fn get(&self, _id: i64) -> Result<Option<Task>, sqlx::Error> {
        todo!("TaskRepo::get")
    }

    /// TODO: overwrite every editable column; keep `id` and `created_at`.
    /// `Ok(None)` when the id does not exist.
    pub async fn replace(&self, _id: i64, _new: &NewTask) -> Result<Option<Task>, sqlx::Error> {
        todo!("TaskRepo::replace")
    }

    /// TODO: `Ok(true)` if a row was deleted.
    pub async fn delete(&self, _id: i64) -> Result<bool, sqlx::Error> {
        todo!("TaskRepo::delete")
    }

    /// TODO: filter by status and tag, sort, paginate, and also return the
    /// total number of matches. Never put user text into the SQL string:
    /// bind values, and map `Sort` to fixed `ORDER BY` fragments (sqlx 0.9
    /// wants `&'static str` SQL; a built string needs `sqlx::AssertSqlSafe`).
    /// Tasks without a due date come last for both due-date sorts.
    pub async fn list(&self, _params: &ListParams) -> Result<Page, sqlx::Error> {
        todo!("TaskRepo::list")
    }
}
