//! The Task model, the storage abstraction and the `/tasks` handlers.

use std::collections::BTreeMap;
use std::sync::Mutex;

use axum::body::Bytes;
use axum::extract::{Path, State};
use axum::http::{HeaderValue, StatusCode, header};
use axum::response::{IntoResponse, Response};
use axum::routing::get;
use axum::{Json, Router};
use serde::Serialize;
use serde_json::{Value, json};

use crate::AppState;

#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum TaskStatus {
    Todo,
    InProgress,
    Done,
}

/// A reduced Task (see the domain spec): enough to show the packaging story.
#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct Task {
    pub id: u64,
    pub title: String,
    pub status: TaskStatus,
}

/// Storage behind a trait so tests can plug in a fake.
pub trait TaskStore: Send + Sync {
    fn create(&self, title: String) -> Task;
    fn get(&self, id: u64) -> Option<Task>;
    /// Ordered by id.
    fn list(&self) -> Vec<Task>;
}

#[derive(Default)]
pub struct MemoryStore {
    inner: Mutex<BTreeMap<u64, Task>>,
}

impl TaskStore for MemoryStore {
    fn create(&self, title: String) -> Task {
        let mut tasks = self.inner.lock().unwrap_or_else(|e| e.into_inner());
        let id = tasks.last_key_value().map_or(1, |(id, _)| id + 1);
        let task = Task {
            id,
            title,
            status: TaskStatus::Todo,
        };
        tasks.insert(id, task.clone());
        task
    }

    fn get(&self, id: u64) -> Option<Task> {
        let tasks = self.inner.lock().unwrap_or_else(|e| e.into_inner());
        tasks.get(&id).cloned()
    }

    fn list(&self) -> Vec<Task> {
        let tasks = self.inner.lock().unwrap_or_else(|e| e.into_inner());
        tasks.values().cloned().collect()
    }
}

pub fn routes() -> Router<AppState> {
    Router::new()
        .route("/tasks", get(list_tasks).post(create_task))
        .route("/tasks/{id}", get(get_task))
}

#[derive(Debug)]
pub enum ApiError {
    BadRequest(String),
    Validation(&'static str, &'static str),
    NotFound,
}

impl IntoResponse for ApiError {
    fn into_response(self) -> Response {
        let (status, error) = match self {
            ApiError::BadRequest(message) => (
                StatusCode::BAD_REQUEST,
                json!({ "code": "bad_request", "message": message }),
            ),
            ApiError::Validation(field, message) => (
                StatusCode::UNPROCESSABLE_ENTITY,
                json!({
                    "code": "validation_failed",
                    "message": "invalid task",
                    "fields": { field: message },
                }),
            ),
            ApiError::NotFound => (
                StatusCode::NOT_FOUND,
                json!({ "code": "not_found", "message": "not found" }),
            ),
        };
        (status, Json(json!({ "error": error }))).into_response()
    }
}

// Raw bytes instead of `Json<T>` so a malformed body (400) and an invalid
// task (422) get the error envelope instead of the extractor's plain text.
async fn create_task(State(state): State<AppState>, body: Bytes) -> Result<Response, ApiError> {
    let value: Value = serde_json::from_slice(&body)
        .map_err(|e| ApiError::BadRequest(format!("malformed JSON: {e}")))?;
    let title = value
        .get("title")
        .and_then(Value::as_str)
        .map(str::trim)
        .filter(|t| (1..=200).contains(&t.chars().count()))
        .ok_or(ApiError::Validation("title", "must be 1-200 characters"))?;
    let task = state.store.create(title.to_owned());
    tracing::info!(id = task.id, "task created");
    let mut response = (StatusCode::CREATED, Json(&task)).into_response();
    let location = HeaderValue::from_str(&format!("/tasks/{}", task.id))
        .expect("a path made of digits is a valid header value");
    response.headers_mut().insert(header::LOCATION, location);
    Ok(response)
}

// `Path<u64>` would answer a non-numeric id with plain text; "not a number"
// simply means "no such task".
async fn get_task(
    State(state): State<AppState>,
    Path(id): Path<String>,
) -> Result<Json<Task>, ApiError> {
    id.parse()
        .ok()
        .and_then(|id| state.store.get(id))
        .map(Json)
        .ok_or(ApiError::NotFound)
}

#[derive(Serialize)]
struct TaskList {
    items: Vec<Task>,
    total: usize,
}

async fn list_tasks(State(state): State<AppState>) -> Json<TaskList> {
    let items = state.store.list();
    Json(TaskList {
        total: items.len(),
        items,
    })
}
