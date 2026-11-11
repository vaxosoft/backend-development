//! Exercise 2: PATCH semantics - "absent", "null" and "a value" are three different things.
//!
//! * field absent        -> leave the task unchanged   (`None`)
//! * `"due_date": null`  -> clear the value            (`Some(None)`)
//! * `"due_date": "..."` -> set the value              (`Some(Some(date))`)
//!
//! serde collapses `null` and "absent" for a plain `Option<Option<T>>`, so
//! `TaskPatch` uses `deserialize_with = "double_option"`. Implement it, then
//! implement `Task::apply`.

use chrono::NaiveDate;
use serde::{Deserialize, Deserializer, Serialize};

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum Status {
    Todo,
    InProgress,
    Done,
}

#[derive(Debug, Clone, PartialEq)]
pub struct Task {
    pub id: i64,
    pub title: String,
    pub status: Status,
    pub due_date: Option<NaiveDate>,
    pub tags: Vec<String>,
}

/// Body of `PATCH /tasks/{id}`: only these four fields may be changed.
#[derive(Debug, Default, PartialEq, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct TaskPatch {
    pub title: Option<String>,
    pub status: Option<Status>,
    #[serde(default, deserialize_with = "double_option")]
    pub due_date: Option<Option<NaiveDate>>,
    pub tags: Option<Vec<String>>,
}

/// Wraps whatever is present (including `null`) in an outer `Some`.
/// An absent field never reaches this function; `#[serde(default)]` yields `None`.
pub fn double_option<'de, D, T>(deserializer: D) -> Result<Option<Option<T>>, D::Error>
where
    D: Deserializer<'de>,
    T: Deserialize<'de>,
{
    let _ = deserializer;
    todo!("deserialize an Option<T> and wrap it in Some")
}

impl Task {
    pub fn apply(&mut self, patch: TaskPatch) {
        let _ = patch;
        todo!("copy over only the fields the patch carries")
    }
}
