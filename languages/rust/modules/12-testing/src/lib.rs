//! The small Task Tracker service that the testing examples exercise.
//!
//! ```
//! use m12_testing::slugify;
//! assert_eq!(slugify("  Write the Docs!  "), "write-the-docs");
//! ```

use std::collections::BTreeMap;
use std::fmt;
use std::str::FromStr;
use std::sync::{Arc, Mutex};

use axum::extract::rejection::JsonRejection;
use axum::extract::{Path, Query, State};
use axum::http::{HeaderValue, StatusCode, header};
use axum::response::{IntoResponse, Response};
use axum::routing::get;
use axum::{Json, Router};
use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use serde_json::json;

/// Lowercase, ASCII-alphanumeric words joined by single dashes.
pub fn slugify(title: &str) -> String {
    let mut out = String::with_capacity(title.len());
    for c in title.chars() {
        if c.is_ascii_alphanumeric() {
            out.push(c.to_ascii_lowercase());
        } else if !out.is_empty() && !out.ends_with('-') {
            out.push('-');
        }
    }
    while out.ends_with('-') {
        out.pop();
    }
    out
}

#[derive(Debug, PartialEq, Eq, thiserror::Error)]
pub enum ValidationError {
    #[error("title is required")]
    TitleEmpty,
    #[error("title must be at most 200 characters")]
    TitleTooLong,
}

/// Trims and checks the title (1-200 characters).
pub fn validate_title(raw: &str) -> Result<String, ValidationError> {
    let title = raw.trim();
    match title.chars().count() {
        0 => Err(ValidationError::TitleEmpty),
        1..=200 => Ok(title.to_string()),
        _ => Err(ValidationError::TitleTooLong),
    }
}

#[derive(Clone, Copy, Debug, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum Status {
    Todo,
    InProgress,
    Done,
}

impl FromStr for Status {
    type Err = String;

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        match s {
            "todo" => Ok(Self::Todo),
            "in_progress" => Ok(Self::InProgress),
            "done" => Ok(Self::Done),
            other => Err(format!("unknown status {other:?}")),
        }
    }
}

impl fmt::Display for Status {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(match self {
            Self::Todo => "todo",
            Self::InProgress => "in_progress",
            Self::Done => "done",
        })
    }
}

#[derive(Clone, Debug, PartialEq, Eq, Serialize)]
pub struct Task {
    pub id: u64,
    pub title: String,
    pub status: Status,
    pub created_at: DateTime<Utc>,
}

/// Time as a dependency: production uses [`SystemClock`], tests use a fake.
pub trait Clock: Send + Sync {
    fn now(&self) -> DateTime<Utc>;
}

pub struct SystemClock;

impl Clock for SystemClock {
    fn now(&self) -> DateTime<Utc> {
        Utc::now()
    }
}

#[derive(Debug, PartialEq, Eq, thiserror::Error)]
#[error("store failure: {0}")]
pub struct StoreError(pub String);

/// Storage boundary of the service; handlers only see this trait.
pub trait TaskStore: Send + Sync {
    fn insert(&self, title: &str, created_at: DateTime<Utc>) -> Result<Task, StoreError>;
    fn get(&self, id: u64) -> Result<Option<Task>, StoreError>;
    /// Oldest first (by id).
    fn list(&self, status: Option<Status>) -> Result<Vec<Task>, StoreError>;
    fn delete(&self, id: u64) -> Result<bool, StoreError>;
}

#[derive(Default)]
pub struct MemoryStore {
    inner: Mutex<(u64, BTreeMap<u64, Task>)>,
}

impl MemoryStore {
    pub fn new() -> Self {
        Self::default()
    }
}

impl TaskStore for MemoryStore {
    fn insert(&self, title: &str, created_at: DateTime<Utc>) -> Result<Task, StoreError> {
        let mut guard = self
            .inner
            .lock()
            .map_err(|_| StoreError("poisoned".into()))?;
        guard.0 += 1;
        let task = Task {
            id: guard.0,
            title: title.to_string(),
            status: Status::Todo,
            created_at,
        };
        guard.1.insert(task.id, task.clone());
        Ok(task)
    }

    fn get(&self, id: u64) -> Result<Option<Task>, StoreError> {
        let guard = self
            .inner
            .lock()
            .map_err(|_| StoreError("poisoned".into()))?;
        Ok(guard.1.get(&id).cloned())
    }

    fn list(&self, status: Option<Status>) -> Result<Vec<Task>, StoreError> {
        let guard = self
            .inner
            .lock()
            .map_err(|_| StoreError("poisoned".into()))?;
        Ok(guard
            .1
            .values()
            .filter(|t| status.is_none_or(|s| t.status == s))
            .cloned()
            .collect())
    }

    fn delete(&self, id: u64) -> Result<bool, StoreError> {
        let mut guard = self
            .inner
            .lock()
            .map_err(|_| StoreError("poisoned".into()))?;
        Ok(guard.1.remove(&id).is_some())
    }
}

#[derive(Clone)]
struct AppState {
    store: Arc<dyn TaskStore>,
    clock: Arc<dyn Clock>,
}

enum ApiError {
    BadRequest(String),
    Validation(ValidationError),
    NotFound,
    Internal,
}

impl From<StoreError> for ApiError {
    fn from(_: StoreError) -> Self {
        // details stay in the logs, never in the response
        Self::Internal
    }
}

impl IntoResponse for ApiError {
    fn into_response(self) -> Response {
        let (status, code, message) = match self {
            Self::BadRequest(m) => (StatusCode::BAD_REQUEST, "bad_request", m),
            Self::Validation(e) => (
                StatusCode::UNPROCESSABLE_ENTITY,
                "validation_failed",
                e.to_string(),
            ),
            Self::NotFound => (
                StatusCode::NOT_FOUND,
                "not_found",
                "task not found".to_string(),
            ),
            Self::Internal => (
                StatusCode::INTERNAL_SERVER_ERROR,
                "internal",
                "internal error".to_string(),
            ),
        };
        (
            status,
            Json(json!({"error": {"code": code, "message": message}})),
        )
            .into_response()
    }
}

#[derive(Deserialize)]
struct NewTask {
    title: String,
}

#[derive(Deserialize)]
struct ListQuery {
    status: Option<String>,
}

async fn create(
    State(app): State<AppState>,
    body: Result<Json<NewTask>, JsonRejection>,
) -> Result<Response, ApiError> {
    let Json(new) = body.map_err(|e| ApiError::BadRequest(e.body_text()))?;
    let title = validate_title(&new.title).map_err(ApiError::Validation)?;
    let task = app.store.insert(&title, app.clock.now())?;
    let location =
        HeaderValue::from_str(&format!("/tasks/{}", task.id)).map_err(|_| ApiError::Internal)?;
    let mut resp = (StatusCode::CREATED, Json(task)).into_response();
    resp.headers_mut().insert(header::LOCATION, location);
    Ok(resp)
}

async fn fetch(State(app): State<AppState>, Path(id): Path<u64>) -> Result<Json<Task>, ApiError> {
    app.store.get(id)?.map(Json).ok_or(ApiError::NotFound)
}

async fn list(
    State(app): State<AppState>,
    Query(q): Query<ListQuery>,
) -> Result<Json<serde_json::Value>, ApiError> {
    let status = q
        .status
        .as_deref()
        .map(str::parse::<Status>)
        .transpose()
        .map_err(ApiError::BadRequest)?;
    let items = app.store.list(status)?;
    Ok(Json(json!({"total": items.len(), "items": items})))
}

async fn remove(State(app): State<AppState>, Path(id): Path<u64>) -> Result<StatusCode, ApiError> {
    if app.store.delete(id)? {
        Ok(StatusCode::NO_CONTENT)
    } else {
        Err(ApiError::NotFound)
    }
}

/// Builds the router. Dependencies are injected, so tests can pass fakes.
pub fn app(store: Arc<dyn TaskStore>, clock: Arc<dyn Clock>) -> Router {
    Router::new()
        .route("/tasks", get(list).post(create))
        .route("/tasks/{id}", get(fetch).delete(remove))
        .with_state(AppState { store, clock })
}

#[derive(Debug, thiserror::Error)]
pub enum NotifyError {
    #[error("request failed: {0}")]
    Transport(#[from] reqwest::Error),
    #[error("webhook answered {0}")]
    Status(StatusCode),
}

/// Tells an external system that a task was created. Non-2xx answers are errors.
pub async fn notify_created(
    client: &reqwest::Client,
    url: &str,
    task: &Task,
) -> Result<(), NotifyError> {
    let resp = client
        .post(url)
        .json(&json!({"event": "task.created", "task": task}))
        .send()
        .await?;
    let status = resp.status();
    if status.is_success() {
        Ok(())
    } else {
        Err(NotifyError::Status(
            StatusCode::from_u16(status.as_u16()).unwrap_or(StatusCode::BAD_GATEWAY),
        ))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    // Unit tests live next to the code and can reach private items.
    #[test]
    fn api_error_never_leaks_store_details() {
        let err: ApiError = StoreError("disk /var/db/secret.sqlite is on fire".into()).into();
        assert!(matches!(err, ApiError::Internal));
    }
}
