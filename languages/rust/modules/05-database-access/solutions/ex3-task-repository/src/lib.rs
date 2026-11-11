//! Exercise 3: a repository with dynamic filters, pagination and a total count.
//!
//! `TaskRepo` hides SQL behind domain types (the repository pattern). `create`,
//! `get` and the schema are done. Implement `push_filter`, `list`,
//! `update_status` and `delete`.
//!
//! `list` must build its WHERE clause with `QueryBuilder` and `push_bind`: only
//! the filters that are present may appear in the SQL, and user values are
//! always bound, never concatenated.

use chrono::NaiveDate;
use sqlx::sqlite::SqlitePoolOptions;
use sqlx::{FromRow, QueryBuilder, Sqlite, SqlitePool};

#[derive(Debug, Clone, Copy, PartialEq, Eq, sqlx::Type)]
#[sqlx(type_name = "TEXT", rename_all = "snake_case")]
pub enum Status {
    Todo,
    InProgress,
    Done,
}

#[derive(Debug, Clone)]
pub struct NewTask {
    pub title: String,
    pub status: Status,
    pub due_date: Option<NaiveDate>,
    pub tags: Vec<String>,
}

#[derive(Debug, Clone, PartialEq)]
pub struct Task {
    pub id: i64,
    pub title: String,
    pub status: Status,
    pub due_date: Option<NaiveDate>,
    pub tags: Vec<String>,
}

#[derive(Debug, FromRow)]
struct TaskRecord {
    id: i64,
    title: String,
    status: Status,
    due_date: Option<NaiveDate>,
}

#[derive(Debug, Default, Clone)]
pub struct TaskFilter {
    pub status: Option<Status>,
    pub tag: Option<String>,
    pub limit: Option<i64>,
    pub offset: Option<i64>,
}

impl TaskFilter {
    /// Default 20, clamped to 1..=100.
    pub fn effective_limit(&self) -> i64 {
        self.limit.unwrap_or(20).clamp(1, 100)
    }

    pub fn effective_offset(&self) -> i64 {
        self.offset.unwrap_or(0).max(0)
    }
}

#[derive(Debug)]
pub struct Page {
    pub items: Vec<Task>,
    /// Matches of the filter ignoring limit/offset.
    pub total: i64,
}

#[derive(Clone)]
pub struct TaskRepo {
    pool: SqlitePool,
}

impl TaskRepo {
    /// In-memory database (single connection) with the schema applied.
    pub async fn in_memory() -> Result<TaskRepo, sqlx::Error> {
        let pool = SqlitePoolOptions::new()
            .max_connections(1)
            .connect("sqlite::memory:")
            .await?;
        for ddl in [
            "CREATE TABLE tasks (
                id       INTEGER PRIMARY KEY AUTOINCREMENT,
                title    TEXT NOT NULL,
                status   TEXT NOT NULL DEFAULT 'todo',
                due_date TEXT
            )",
            "CREATE TABLE task_tags (
                task_id INTEGER NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
                tag     TEXT NOT NULL,
                PRIMARY KEY (task_id, tag)
            )",
        ] {
            sqlx::query(ddl).execute(&pool).await?;
        }
        Ok(TaskRepo { pool })
    }

    /// Escape hatch for tests that inspect tables directly.
    pub fn pool(&self) -> &SqlitePool {
        &self.pool
    }

    pub async fn create(&self, new: NewTask) -> Result<Task, sqlx::Error> {
        let mut tx = self.pool.begin().await?;
        let id: i64 = sqlx::query_scalar(
            "INSERT INTO tasks (title, status, due_date) VALUES (?, ?, ?) RETURNING id",
        )
        .bind(&new.title)
        .bind(new.status)
        .bind(new.due_date)
        .fetch_one(&mut *tx)
        .await?;
        for tag in &new.tags {
            sqlx::query("INSERT INTO task_tags (task_id, tag) VALUES (?, ?)")
                .bind(id)
                .bind(tag)
                .execute(&mut *tx)
                .await?;
        }
        tx.commit().await?;
        Ok(Task {
            id,
            title: new.title,
            status: new.status,
            due_date: new.due_date,
            tags: new.tags,
        })
    }

    pub async fn get(&self, id: i64) -> Result<Option<Task>, sqlx::Error> {
        let record: Option<TaskRecord> =
            sqlx::query_as("SELECT id, title, status, due_date FROM tasks WHERE id = ?")
                .bind(id)
                .fetch_optional(&self.pool)
                .await?;
        match record {
            Some(record) => Ok(Some(self.with_tags(record).await?)),
            None => Ok(None),
        }
    }

    async fn with_tags(&self, record: TaskRecord) -> Result<Task, sqlx::Error> {
        let tags = sqlx::query_scalar("SELECT tag FROM task_tags WHERE task_id = ? ORDER BY tag")
            .bind(record.id)
            .fetch_all(&self.pool)
            .await?;
        Ok(Task {
            id: record.id,
            title: record.title,
            status: record.status,
            due_date: record.due_date,
            tags,
        })
    }

    /// Newest first (`id DESC`). Applies `status` and `tag` filters when present.
    pub async fn list(&self, filter: &TaskFilter) -> Result<Page, sqlx::Error> {
        let mut select =
            QueryBuilder::new("SELECT id, title, status, due_date FROM tasks WHERE 1 = 1");
        push_filter(&mut select, filter);
        select
            .push(" ORDER BY id DESC LIMIT ")
            .push_bind(filter.effective_limit())
            .push(" OFFSET ")
            .push_bind(filter.effective_offset());
        let records: Vec<TaskRecord> = select.build_query_as().fetch_all(&self.pool).await?;

        let mut items = Vec::with_capacity(records.len());
        for record in records {
            items.push(self.with_tags(record).await?);
        }

        let mut count = QueryBuilder::new("SELECT COUNT(*) FROM tasks WHERE 1 = 1");
        push_filter(&mut count, filter);
        let total = count.build_query_scalar().fetch_one(&self.pool).await?;
        Ok(Page { items, total })
    }

    /// `Ok(None)` when the task does not exist.
    pub async fn update_status(
        &self,
        id: i64,
        status: Status,
    ) -> Result<Option<Task>, sqlx::Error> {
        let result = sqlx::query("UPDATE tasks SET status = ? WHERE id = ?")
            .bind(status)
            .bind(id)
            .execute(&self.pool)
            .await?;
        if result.rows_affected() == 0 {
            return Ok(None);
        }
        self.get(id).await
    }

    /// `true` when a row was deleted. Tags go away with it (ON DELETE CASCADE).
    pub async fn delete(&self, id: i64) -> Result<bool, sqlx::Error> {
        let result = sqlx::query("DELETE FROM tasks WHERE id = ?")
            .bind(id)
            .execute(&self.pool)
            .await?;
        Ok(result.rows_affected() > 0)
    }
}

/// Appends ` AND ...` conditions (the query must already contain a WHERE).
fn push_filter(builder: &mut QueryBuilder<Sqlite>, filter: &TaskFilter) {
    if let Some(status) = filter.status {
        builder.push(" AND status = ").push_bind(status);
    }
    if let Some(tag) = &filter.tag {
        builder
            .push(" AND EXISTS (SELECT 1 FROM task_tags WHERE task_id = tasks.id AND tag = ")
            .push_bind(tag.as_str())
            .push(")");
    }
}
