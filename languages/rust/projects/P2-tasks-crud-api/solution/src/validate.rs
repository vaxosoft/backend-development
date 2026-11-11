//! Validation and query parsing.

use std::collections::HashSet;

use chrono::NaiveDate;

use crate::error::ApiError;
use crate::model::{ListParams, NewTask, RawListQuery, RawPatch, RawTask, Sort, Status, TaskPatch};

pub const MAX_TITLE_CHARS: usize = 200;
pub const MAX_TAGS: usize = 10;
pub const MAX_TAG_CHARS: usize = 30;
pub const DEFAULT_LIMIT: i64 = 20;
pub const MAX_LIMIT: i64 = 100;

type Check<T> = Result<T, &'static str>;

fn check_title(raw: &str) -> Check<String> {
    let title = raw.trim();
    if (1..=MAX_TITLE_CHARS).contains(&title.chars().count()) {
        Ok(title.to_owned())
    } else {
        Err("must be 1-200 characters")
    }
}

fn check_status(raw: &str) -> Check<Status> {
    Status::parse(raw).ok_or("must be one of todo, in_progress, done")
}

fn check_due_date(raw: &str) -> Check<NaiveDate> {
    const MESSAGE: &str = "must be a date in YYYY-MM-DD format";
    // chrono also accepts "2030-1-5"; the wire format is strict.
    if raw.len() != 10 {
        return Err(MESSAGE);
    }
    NaiveDate::parse_from_str(raw, "%Y-%m-%d").map_err(|_| MESSAGE)
}

fn check_tags(tags: Vec<String>) -> Check<Vec<String>> {
    if tags.len() > MAX_TAGS {
        return Err("must contain at most 10 tags");
    }
    let mut seen = HashSet::new();
    for tag in &tags {
        if !(1..=MAX_TAG_CHARS).contains(&tag.chars().count()) {
            return Err("each tag must be 1-30 characters");
        }
        if *tag != tag.to_lowercase() {
            return Err("tags must be lowercase");
        }
        if !seen.insert(tag.as_str()) {
            return Err("tags must be unique");
        }
    }
    Ok(tags)
}

pub fn validate_new(raw: RawTask) -> Result<NewTask, ApiError> {
    let title = raw
        .title
        .as_deref()
        .ok_or("is required")
        .and_then(check_title);
    let status = raw.status.as_deref().map_or(Ok(Status::Todo), check_status);
    let due_date = raw.due_date.as_deref().map(check_due_date).transpose();
    let tags = check_tags(raw.tags.unwrap_or_default());

    match (title, status, due_date, tags) {
        (Ok(title), Ok(status), Ok(due_date), Ok(tags)) => Ok(NewTask {
            title,
            status,
            due_date,
            tags,
            owner_id: raw.owner_id,
        }),
        (title, status, due_date, tags) => Err(ApiError::validation([
            ("title", title.err()),
            ("status", status.err()),
            ("due_date", due_date.err()),
            ("tags", tags.err()),
        ])),
    }
}

pub fn validate_patch(raw: RawPatch) -> Result<TaskPatch, ApiError> {
    let title = raw.title.as_deref().map(check_title).transpose();
    let status = raw.status.as_deref().map(check_status).transpose();
    let due_date = raw
        .due_date
        .map(|value| value.as_deref().map(check_due_date).transpose())
        .transpose();
    let tags = raw.tags.map(check_tags).transpose();

    match (title, status, due_date, tags) {
        (Ok(title), Ok(status), Ok(due_date), Ok(tags)) => Ok(TaskPatch {
            title,
            status,
            due_date,
            tags,
        }),
        (title, status, due_date, tags) => Err(ApiError::validation([
            ("title", title.err()),
            ("status", status.err()),
            ("due_date", due_date.err()),
            ("tags", tags.err()),
        ])),
    }
}

pub fn apply_patch(task: &mut NewTask, patch: TaskPatch) {
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
}

fn non_empty(value: Option<String>) -> Option<String> {
    value.filter(|v| !v.is_empty())
}

pub fn parse_list(raw: RawListQuery) -> Result<ListParams, ApiError> {
    let bad = |message: &str| ApiError::BadRequest(message.to_owned());

    let status = non_empty(raw.status)
        .map(|s| Status::parse(&s).ok_or_else(|| bad("status must be todo, in_progress or done")))
        .transpose()?;
    let limit = match non_empty(raw.limit) {
        None => DEFAULT_LIMIT,
        Some(s) => s
            .parse::<i64>()
            .ok()
            .filter(|n| *n >= 1)
            .ok_or_else(|| bad("limit must be a positive integer"))?
            .min(MAX_LIMIT),
    };
    let offset = match non_empty(raw.offset) {
        None => 0,
        Some(s) => s
            .parse::<i64>()
            .ok()
            .filter(|n| *n >= 0)
            .ok_or_else(|| bad("offset must be a non-negative integer"))?,
    };
    let sort = match non_empty(raw.sort).as_deref() {
        None | Some("-created_at") => Sort::CreatedDesc,
        Some("created_at") => Sort::CreatedAsc,
        Some("due_date") => Sort::DueDateAsc,
        Some("-due_date") => Sort::DueDateDesc,
        Some(_) => {
            return Err(bad(
                "sort must be created_at, -created_at, due_date or -due_date",
            ));
        }
    };

    Ok(ListParams {
        status,
        tag: non_empty(raw.tag),
        limit,
        offset,
        sort,
    })
}

pub fn parse_id(raw: &str) -> Result<i64, ApiError> {
    raw.parse::<i64>()
        .ok()
        .filter(|id| *id >= 1)
        .ok_or(ApiError::NotFound)
}
