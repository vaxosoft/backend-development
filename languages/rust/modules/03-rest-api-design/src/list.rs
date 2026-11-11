//! Filtering, sorting and pagination for `GET /tasks`.

use serde::{Deserialize, Serialize};

use crate::error::ApiError;
use crate::tasks::{Status, Task};

pub const DEFAULT_LIMIT: usize = 20;
pub const MAX_LIMIT: usize = 100;

/// Raw query string. A value that is not a number (`limit=abc`) is rejected by
/// the `ApiQuery` extractor before the handler runs.
#[derive(Debug, Default, Deserialize)]
pub struct ListQuery {
    pub status: Option<String>,
    pub tag: Option<String>,
    pub limit: Option<usize>,
    pub offset: Option<usize>,
    pub sort: Option<String>,
}

#[derive(Debug, Serialize)]
pub struct Page<T> {
    pub items: Vec<T>,
    pub total: usize,
    pub limit: usize,
    pub offset: usize,
}

#[derive(Debug, Clone, Copy)]
enum SortKey {
    CreatedAt,
    DueDate,
}

fn parse_sort(raw: &str) -> Result<(SortKey, bool), ApiError> {
    let (name, descending) = match raw.strip_prefix('-') {
        Some(name) => (name, true),
        None => (raw, false),
    };
    let key = match name {
        "created_at" => SortKey::CreatedAt,
        "due_date" => SortKey::DueDate,
        _ => {
            return Err(ApiError::BadRequest(
                "sort must be one of created_at, -created_at, due_date, -due_date".to_string(),
            ));
        }
    };
    Ok((key, descending))
}

/// Filters, sorts (default `-created_at`) and slices `tasks`.
///
/// `limit` defaults to 20 and is clamped to 1..=100; `total` counts matches
/// before slicing, so clients can build "page 3 of 7".
pub fn list_tasks(tasks: &[Task], query: &ListQuery) -> Result<Page<Task>, ApiError> {
    let status = match query.status.as_deref() {
        Some(raw) => Some(Status::parse(raw).ok_or_else(|| {
            ApiError::BadRequest("status must be one of todo, in_progress, done".to_string())
        })?),
        None => None,
    };
    let (key, descending) = parse_sort(query.sort.as_deref().unwrap_or("-created_at"))?;

    let mut matches: Vec<&Task> = tasks
        .iter()
        .filter(|t| status.is_none_or(|s| t.status == s))
        .filter(|t| query.tag.as_ref().is_none_or(|tag| t.tags.contains(tag)))
        .collect();

    matches.sort_by(|a, b| {
        let order = match key {
            SortKey::CreatedAt => a.created_at.cmp(&b.created_at),
            // Tasks without a due date always go last, whatever the direction.
            SortKey::DueDate => match (&a.due_date, &b.due_date) {
                (Some(x), Some(y)) if descending => y.cmp(x),
                (Some(x), Some(y)) => x.cmp(y),
                (Some(_), None) => std::cmp::Ordering::Less,
                (None, Some(_)) => std::cmp::Ordering::Greater,
                (None, None) => std::cmp::Ordering::Equal,
            },
        };
        let order = match key {
            SortKey::CreatedAt if descending => order.reverse(),
            _ => order,
        };
        order.then(a.id.cmp(&b.id))
    });

    let limit = query.limit.unwrap_or(DEFAULT_LIMIT).clamp(1, MAX_LIMIT);
    let offset = query.offset.unwrap_or(0);
    let total = matches.len();
    let items = matches
        .into_iter()
        .skip(offset)
        .take(limit)
        .cloned()
        .collect();
    Ok(Page {
        items,
        total,
        limit,
        offset,
    })
}
