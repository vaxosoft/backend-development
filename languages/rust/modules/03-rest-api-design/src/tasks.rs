//! Task Tracker domain types, validation and an in-memory store.

use std::collections::BTreeMap;

use serde::{Deserialize, Deserializer, Serialize};

use crate::error::{ApiError, FieldErrors};

/// Stand-in for a real clock (see module 13) so example output is reproducible.
pub const FIXED_NOW: &str = "2030-01-01T09:00:00Z";

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum Status {
    Todo,
    InProgress,
    Done,
}

impl Status {
    pub fn parse(text: &str) -> Option<Status> {
        match text {
            "todo" => Some(Status::Todo),
            "in_progress" => Some(Status::InProgress),
            "done" => Some(Status::Done),
            _ => None,
        }
    }
}

#[derive(Debug, Clone, PartialEq, Serialize)]
pub struct Task {
    pub id: u32,
    pub title: String,
    pub status: Status,
    pub due_date: Option<String>,
    pub tags: Vec<String>,
    pub owner_id: Option<u32>,
    pub created_at: String,
}

/// Body of `POST` / `PUT`. Every field is optional here so that *we* decide what is
/// missing and report it per field, instead of serde failing with one opaque message.
#[derive(Debug, Default, Deserialize)]
pub struct TaskInput {
    pub title: Option<String>,
    pub status: Option<String>,
    pub due_date: Option<String>,
    pub tags: Option<Vec<String>>,
    pub owner_id: Option<i64>,
}

/// A fully validated, normalised task (no id or timestamp yet).
#[derive(Debug, Clone, PartialEq)]
pub struct NewTask {
    pub title: String,
    pub status: Status,
    pub due_date: Option<String>,
    pub tags: Vec<String>,
    pub owner_id: Option<u32>,
}

/// Body of `PATCH`. `due_date` distinguishes three states:
/// absent (`None`), explicit `null` (`Some(None)`) and a value (`Some(Some(..))`).
#[derive(Debug, Default, Deserialize)]
pub struct PatchInput {
    pub title: Option<String>,
    pub status: Option<String>,
    #[serde(default, deserialize_with = "present")]
    pub due_date: Option<Option<String>>,
    pub tags: Option<Vec<String>>,
}

fn present<'de, D, T>(deserializer: D) -> Result<Option<Option<T>>, D::Error>
where
    D: Deserializer<'de>,
    T: Deserialize<'de>,
{
    Option::<T>::deserialize(deserializer).map(Some)
}

#[derive(Debug, Clone, PartialEq)]
pub struct TaskPatch {
    pub title: Option<String>,
    pub status: Option<Status>,
    pub due_date: Option<Option<String>>,
    pub tags: Option<Vec<String>>,
}

fn check_title(raw: &str) -> Result<String, &'static str> {
    let title = raw.trim();
    if (1..=200).contains(&title.chars().count()) {
        Ok(title.to_string())
    } else {
        Err("must be 1-200 characters")
    }
}

fn check_status(raw: &str) -> Result<Status, &'static str> {
    Status::parse(raw).ok_or("must be one of todo, in_progress, done")
}

fn check_due_date(raw: &str) -> Result<String, &'static str> {
    if is_valid_date(raw) {
        Ok(raw.to_string())
    } else {
        Err("must be a calendar date in YYYY-MM-DD format")
    }
}

fn check_tags(tags: &[String]) -> Result<Vec<String>, &'static str> {
    if tags.len() > 10 {
        return Err("at most 10 tags");
    }
    for (i, tag) in tags.iter().enumerate() {
        if !(1..=30).contains(&tag.chars().count()) {
            return Err("each tag must be 1-30 characters");
        }
        if *tag != tag.to_lowercase() {
            return Err("tags must be lowercase");
        }
        if tags[..i].contains(tag) {
            return Err("tags must be unique");
        }
    }
    Ok(tags.to_vec())
}

/// `YYYY-MM-DD` that exists on the calendar (leap years included).
pub fn is_valid_date(text: &str) -> bool {
    let bytes = text.as_bytes();
    let shape_ok = bytes.len() == 10
        && bytes[4] == b'-'
        && bytes[7] == b'-'
        && bytes
            .iter()
            .enumerate()
            .all(|(i, b)| i == 4 || i == 7 || b.is_ascii_digit());
    if !shape_ok {
        return false;
    }
    let (Ok(year), Ok(month), Ok(day)) = (
        text[..4].parse::<u32>(),
        text[5..7].parse::<u32>(),
        text[8..].parse::<u32>(),
    ) else {
        return false;
    };
    let leap = (year % 4 == 0 && year % 100 != 0) || year % 400 == 0;
    let days = match month {
        1 | 3 | 5 | 7 | 8 | 10 | 12 => 31,
        4 | 6 | 9 | 11 => 30,
        2 if leap => 29,
        2 => 28,
        _ => return false,
    };
    (1..=days).contains(&day)
}

/// Records the problem for `field` and yields `None`, or passes the value through.
fn collect<T>(fields: &mut FieldErrors, field: &str, result: Result<T, &'static str>) -> Option<T> {
    match result {
        Ok(value) => Some(value),
        Err(problem) => {
            fields.insert(field.to_string(), problem.to_string());
            None
        }
    }
}

fn check_owner(id: i64) -> Result<u32, &'static str> {
    u32::try_from(id)
        .ok()
        .filter(|id| *id >= 1)
        .ok_or("must be a positive integer")
}

/// Validates a create/replace body; every violated rule is reported at once (422).
pub fn validate_task(input: TaskInput) -> Result<NewTask, ApiError> {
    let mut fields = FieldErrors::new();
    let title = match input.title.as_deref() {
        Some(raw) => collect(&mut fields, "title", check_title(raw)),
        None => collect(&mut fields, "title", Err("is required")),
    };
    let status = match input.status.as_deref() {
        Some(raw) => collect(&mut fields, "status", check_status(raw)),
        None => Some(Status::Todo),
    };
    let due_date = match input.due_date.as_deref() {
        Some(raw) => collect(&mut fields, "due_date", check_due_date(raw)).map(Some),
        None => Some(None),
    };
    let tags = match input.tags.as_deref() {
        Some(raw) => collect(&mut fields, "tags", check_tags(raw)),
        None => Some(vec![]),
    };
    let owner_id = match input.owner_id {
        Some(raw) => collect(&mut fields, "owner_id", check_owner(raw)).map(Some),
        None => Some(None),
    };
    match (title, status, due_date, tags, owner_id) {
        (Some(title), Some(status), Some(due_date), Some(tags), Some(owner_id)) => Ok(NewTask {
            title,
            status,
            due_date,
            tags,
            owner_id,
        }),
        _ => Err(ApiError::validation("invalid task", fields)),
    }
}

/// Validates a PATCH body: only the fields that are present are checked.
pub fn validate_patch(input: PatchInput) -> Result<TaskPatch, ApiError> {
    let mut fields = FieldErrors::new();
    let title = input
        .title
        .as_deref()
        .and_then(|raw| collect(&mut fields, "title", check_title(raw)));
    let status = input
        .status
        .as_deref()
        .and_then(|raw| collect(&mut fields, "status", check_status(raw)));
    let due_date = match input.due_date {
        Some(Some(raw)) => {
            collect(&mut fields, "due_date", check_due_date(&raw)).map(|d| Some(Some(d)))
        }
        Some(None) => Some(Some(None)),
        None => Some(None),
    };
    let tags = input
        .tags
        .as_deref()
        .and_then(|raw| collect(&mut fields, "tags", check_tags(raw)));
    if !fields.is_empty() {
        return Err(ApiError::validation("invalid task", fields));
    }
    Ok(TaskPatch {
        title,
        status,
        due_date: due_date.flatten(),
        tags,
    })
}

#[derive(Default)]
pub struct TaskStore {
    next_id: u32,
    tasks: BTreeMap<u32, Task>,
}

impl TaskStore {
    pub fn create(&mut self, new: NewTask) -> Task {
        self.next_id += 1;
        let task = Task {
            id: self.next_id,
            title: new.title,
            status: new.status,
            due_date: new.due_date,
            tags: new.tags,
            owner_id: new.owner_id,
            created_at: FIXED_NOW.to_string(),
        };
        self.tasks.insert(task.id, task.clone());
        task
    }

    pub fn get(&self, id: u32) -> Option<Task> {
        self.tasks.get(&id).cloned()
    }

    pub fn all(&self) -> Vec<Task> {
        self.tasks.values().cloned().collect()
    }

    /// Full replace: everything but `id` and `created_at` comes from `new`.
    pub fn replace(&mut self, id: u32, new: NewTask) -> Option<Task> {
        let task = self.tasks.get_mut(&id)?;
        task.title = new.title;
        task.status = new.status;
        task.due_date = new.due_date;
        task.tags = new.tags;
        task.owner_id = new.owner_id;
        Some(task.clone())
    }

    pub fn patch(&mut self, id: u32, patch: TaskPatch) -> Option<Task> {
        let task = self.tasks.get_mut(&id)?;
        if let Some(title) = patch.title {
            task.title = title;
        }
        if let Some(status) = patch.status {
            task.status = status;
        }
        if let Some(due_date) = patch.due_date {
            task.due_date = due_date;
        }
        if let Some(tags) = patch.tags {
            task.tags = tags;
        }
        Some(task.clone())
    }

    pub fn delete(&mut self, id: u32) -> bool {
        self.tasks.remove(&id).is_some()
    }
}
