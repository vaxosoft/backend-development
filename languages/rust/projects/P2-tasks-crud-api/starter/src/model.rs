//! Domain types and the wire format. Nothing to implement here.

use chrono::{DateTime, NaiveDate, Utc};
use serde::{Deserialize, Deserializer, Serialize};

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum Status {
    Todo,
    InProgress,
    Done,
}

impl Status {
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Todo => "todo",
            Self::InProgress => "in_progress",
            Self::Done => "done",
        }
    }

    pub fn parse(value: &str) -> Option<Self> {
        match value {
            "todo" => Some(Self::Todo),
            "in_progress" => Some(Self::InProgress),
            "done" => Some(Self::Done),
            _ => None,
        }
    }
}

/// A stored task, exactly as it is serialized to clients.
#[derive(Debug, Clone, Serialize)]
pub struct Task {
    pub id: i64,
    pub title: String,
    pub status: Status,
    pub due_date: Option<NaiveDate>,
    pub tags: Vec<String>,
    pub owner_id: Option<i64>,
    pub created_at: DateTime<Utc>,
}

/// Validated data for creating or replacing a task (no id, no created_at).
#[derive(Debug, Clone)]
pub struct NewTask {
    pub title: String,
    pub status: Status,
    pub due_date: Option<NaiveDate>,
    pub tags: Vec<String>,
    pub owner_id: Option<i64>,
}

impl From<Task> for NewTask {
    fn from(task: Task) -> Self {
        Self {
            title: task.title,
            status: task.status,
            due_date: task.due_date,
            tags: task.tags,
            owner_id: task.owner_id,
        }
    }
}

/// Body of `POST /tasks` and `PUT /tasks/{id}` before validation.
/// Strings stay strings so that a bad value becomes a 422, not a 400.
#[derive(Debug, Deserialize)]
pub struct RawTask {
    pub title: Option<String>,
    pub status: Option<String>,
    pub due_date: Option<String>,
    pub tags: Option<Vec<String>>,
    pub owner_id: Option<i64>,
}

/// Body of `PATCH /tasks/{id}` before validation.
/// `due_date`: `None` = absent, `Some(None)` = explicit `null`.
#[derive(Debug, Deserialize)]
pub struct RawPatch {
    pub title: Option<String>,
    pub status: Option<String>,
    #[serde(default, deserialize_with = "present_even_if_null")]
    pub due_date: Option<Option<String>>,
    pub tags: Option<Vec<String>>,
}

/// Any value that is present, `null` included, becomes `Some(..)`.
fn present_even_if_null<'de, D, T>(deserializer: D) -> Result<Option<Option<T>>, D::Error>
where
    D: Deserializer<'de>,
    T: Deserialize<'de>,
{
    Option::<T>::deserialize(deserializer).map(Some)
}

/// A validated patch: `None` = leave unchanged.
#[derive(Debug, Default)]
pub struct TaskPatch {
    pub title: Option<String>,
    pub status: Option<Status>,
    /// `Some(None)` clears the due date.
    pub due_date: Option<Option<NaiveDate>>,
    pub tags: Option<Vec<String>>,
}

/// Raw query string of `GET /tasks`; every value is parsed by hand.
#[derive(Debug, Default, Deserialize)]
pub struct RawListQuery {
    pub status: Option<String>,
    pub tag: Option<String>,
    pub limit: Option<String>,
    pub offset: Option<String>,
    pub sort: Option<String>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Sort {
    CreatedAsc,
    CreatedDesc,
    DueDateAsc,
    DueDateDesc,
}

#[derive(Debug, Clone)]
pub struct ListParams {
    pub status: Option<Status>,
    pub tag: Option<String>,
    pub limit: i64,
    pub offset: i64,
    pub sort: Sort,
}

/// One page of tasks plus the number of tasks matching the filters.
#[derive(Debug)]
pub struct Page {
    pub items: Vec<Task>,
    pub total: i64,
}

/// The list envelope sent to clients.
#[derive(Debug, Serialize)]
pub struct TaskList {
    pub items: Vec<Task>,
    pub total: i64,
    pub limit: i64,
    pub offset: i64,
}
