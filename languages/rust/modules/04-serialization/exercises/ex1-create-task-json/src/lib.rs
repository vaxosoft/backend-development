//! Exercise 1: make the Task Tracker types speak the shared wire format.
//!
//! The types below compile, but their serde configuration is the default one,
//! so they do not match the wire format yet. Add the right `#[serde(...)]`
//! attributes (no hand-written impls needed).
//!
//! Wire format: `snake_case` enum values (`in_progress`), a create request
//! where only `title` is required, and **unknown fields are rejected**.

use chrono::{DateTime, NaiveDate, Utc};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Copy, PartialEq, Eq, Default, Serialize, Deserialize)]
pub enum Status {
    #[default]
    Todo,
    InProgress,
    Done,
}

/// Body of `POST /tasks`.
#[derive(Debug, Clone, PartialEq, Deserialize)]
pub struct CreateTask {
    pub title: String,
    pub status: Status,
    pub due_date: Option<NaiveDate>,
    pub tags: Vec<String>,
    pub owner_id: Option<i64>,
}

/// A stored task as returned by the API.
#[derive(Debug, Clone, PartialEq, Serialize)]
pub struct Task {
    pub id: i64,
    pub title: String,
    pub status: Status,
    pub due_date: Option<NaiveDate>,
    pub tags: Vec<String>,
    pub owner_id: Option<i64>,
    pub created_at: DateTime<Utc>,
}

impl Task {
    pub fn from_create(id: i64, created_at: DateTime<Utc>, input: CreateTask) -> Task {
        Task {
            id,
            title: input.title,
            status: input.status,
            due_date: input.due_date,
            tags: input.tags,
            owner_id: input.owner_id,
            created_at,
        }
    }
}

pub fn parse_create_task(body: &str) -> Result<CreateTask, serde_json::Error> {
    serde_json::from_str(body)
}

pub fn task_to_json(task: &Task) -> String {
    serde_json::to_string(task).expect("Task always serializes")
}
