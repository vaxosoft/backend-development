//! Validation and query parsing. All TODO.

use crate::error::ApiError;
use crate::model::{ListParams, NewTask, RawListQuery, RawPatch, RawTask, TaskPatch};

pub const MAX_TITLE_CHARS: usize = 200;
pub const MAX_TAGS: usize = 10;
pub const MAX_TAG_CHARS: usize = 30;
pub const DEFAULT_LIMIT: i64 = 20;
pub const MAX_LIMIT: i64 = 100;

/// TODO: validate a `POST`/`PUT` body (rules in the README).
///
/// Collect *every* problem, then fail once with `ApiError::validation([...])`
/// keyed by field name (`title`, `status`, `due_date`, `tags`).
/// Defaults: status `todo`, no due date, no tags, no owner.
pub fn validate_new(_raw: RawTask) -> Result<NewTask, ApiError> {
    todo!("validate_new")
}

/// TODO: validate a `PATCH` body. Absent fields stay `None`; the rules for a
/// present field are the same as in `validate_new`.
pub fn validate_patch(_raw: RawPatch) -> Result<TaskPatch, ApiError> {
    todo!("validate_patch")
}

/// TODO: apply a validated patch on top of the current values.
pub fn apply_patch(_task: &mut NewTask, _patch: TaskPatch) {
    todo!("apply_patch")
}

/// TODO: parse the query string of `GET /tasks`. Bad values are
/// `ApiError::BadRequest`; empty values (`?status=`) mean "not given".
pub fn parse_list(_raw: RawListQuery) -> Result<ListParams, ApiError> {
    todo!("parse_list")
}

/// TODO: a path id that is not a positive integer can never exist: `NotFound`.
pub fn parse_id(_raw: &str) -> Result<i64, ApiError> {
    todo!("parse_id")
}
