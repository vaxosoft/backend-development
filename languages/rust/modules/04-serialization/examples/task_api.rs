//! Task Tracker API: JSON and form bodies in, the shared JSON error envelope out.
//!
//! `ADDR=127.0.0.1:8080 cargo run -p m04-serialization --example task_api`

use std::collections::BTreeMap;
use std::fmt::Display;
use std::str::FromStr;
use std::sync::{Arc, Mutex};

use axum::extract::rejection::{FormRejection, JsonRejection};
use axum::extract::{FromRequest, Path, State};
use axum::http::{StatusCode, header};
use axum::response::{IntoResponse, Response};
use axum::routing::{get, post};
use axum::{Form, Json, Router};
use chrono::{DateTime, NaiveDate, TimeZone, Utc};
use serde::{Deserialize, Deserializer, Serialize};
use serde_json::json;

#[derive(Debug, Clone, Copy, Default, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
enum Status {
    #[default]
    Todo,
    InProgress,
    Done,
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct CreateTask {
    title: String,
    #[serde(default)]
    status: Status,
    due_date: Option<NaiveDate>,
    #[serde(default)]
    tags: Vec<String>,
    owner_id: Option<i64>,
}

/// HTML forms send flat `key=value` pairs: tags are comma separated, empty fields mean "absent".
#[derive(Debug, Deserialize)]
struct TaskForm {
    title: String,
    status: Option<Status>,
    #[serde(default, deserialize_with = "empty_as_none")]
    due_date: Option<NaiveDate>,
    tags: Option<String>,
}

fn empty_as_none<'de, D, T>(d: D) -> Result<Option<T>, D::Error>
where
    D: Deserializer<'de>,
    T: FromStr,
    T::Err: Display,
{
    match Option::<String>::deserialize(d)?.as_deref() {
        None | Some("") => Ok(None),
        Some(raw) => raw.parse().map(Some).map_err(serde::de::Error::custom),
    }
}

impl From<TaskForm> for CreateTask {
    fn from(form: TaskForm) -> Self {
        let tags = form
            .tags
            .iter()
            .flat_map(|t| t.split(','))
            .map(str::trim)
            .filter(|t| !t.is_empty())
            .map(str::to_owned)
            .collect();
        CreateTask {
            title: form.title,
            status: form.status.unwrap_or_default(),
            due_date: form.due_date,
            tags,
            owner_id: None,
        }
    }
}

#[derive(Debug, Clone, Serialize)]
struct Task {
    id: i64,
    title: String,
    status: Status,
    due_date: Option<NaiveDate>,
    tags: Vec<String>,
    owner_id: Option<i64>,
    created_at: DateTime<Utc>,
}

/// Collects every problem so the client can fix them all in one round trip.
fn validate(input: &CreateTask) -> BTreeMap<String, String> {
    let mut fields = BTreeMap::new();
    if !(1..=200).contains(&input.title.trim().chars().count()) {
        fields.insert("title".into(), "must be 1-200 characters".into());
    }
    if input.tags.len() > 10 {
        fields.insert("tags".into(), "at most 10 tags".into());
    } else if input
        .tags
        .iter()
        .any(|t| !(1..=30).contains(&t.chars().count()))
    {
        fields.insert("tags".into(), "each tag must be 1-30 characters".into());
    } else if input.tags.iter().any(|t| *t != t.to_lowercase()) {
        fields.insert("tags".into(), "tags must be lowercase".into());
    } else if (1..input.tags.len()).any(|i| input.tags[..i].contains(&input.tags[i])) {
        fields.insert("tags".into(), "tags must be unique".into());
    }
    fields
}

enum ApiError {
    BadRequest(String),
    Invalid(BTreeMap<String, String>),
    NotFound,
}

impl IntoResponse for ApiError {
    fn into_response(self) -> Response {
        let (status, error) = match self {
            ApiError::BadRequest(message) => (
                StatusCode::BAD_REQUEST,
                json!({"code": "bad_request", "message": message}),
            ),
            ApiError::Invalid(fields) => (
                StatusCode::UNPROCESSABLE_ENTITY,
                json!({"code": "validation_failed", "message": "invalid task", "fields": fields}),
            ),
            ApiError::NotFound => (
                StatusCode::NOT_FOUND,
                json!({"code": "not_found", "message": "task not found"}),
            ),
        };
        (status, Json(json!({ "error": error }))).into_response()
    }
}

/// Syntax errors are the client's malformed request (400); well-formed JSON of the
/// wrong shape is invalid data (422).
impl From<JsonRejection> for ApiError {
    fn from(rejection: JsonRejection) -> Self {
        match rejection {
            JsonRejection::JsonDataError(e) => {
                ApiError::Invalid(BTreeMap::from([("body".to_string(), e.body_text())]))
            }
            other => ApiError::BadRequest(other.body_text()),
        }
    }
}

impl From<FormRejection> for ApiError {
    fn from(rejection: FormRejection) -> Self {
        match rejection {
            FormRejection::FailedToDeserializeFormBody(e) => {
                ApiError::Invalid(BTreeMap::from([("form".to_string(), e.body_text())]))
            }
            other => ApiError::BadRequest(other.body_text()),
        }
    }
}

/// `Json<T>` whose rejection is rendered with our error envelope instead of plain text.
#[derive(FromRequest)]
#[from_request(via(Json), rejection(ApiError))]
struct ApiJson<T>(T);

#[derive(FromRequest)]
#[from_request(via(Form), rejection(ApiError))]
struct ApiForm<T>(T);

#[derive(Default)]
struct Store {
    tasks: Mutex<Vec<Task>>,
}

type AppState = Arc<Store>;

fn store(state: &Store, input: CreateTask) -> Result<Response, ApiError> {
    let fields = validate(&input);
    if !fields.is_empty() {
        return Err(ApiError::Invalid(fields));
    }
    let mut tasks = state.tasks.lock().expect("store lock is never poisoned");
    let task = Task {
        id: tasks.len() as i64 + 1,
        title: input.title.trim().to_owned(),
        status: input.status,
        due_date: input.due_date,
        tags: input.tags,
        owner_id: input.owner_id,
        // Fixed clock so the documented output is reproducible; real code injects a clock.
        created_at: Utc.with_ymd_and_hms(2030, 1, 1, 9, 0, 0).unwrap(),
    };
    tasks.push(task.clone());
    let location = [(header::LOCATION, format!("/tasks/{}", task.id))];
    Ok((StatusCode::CREATED, location, Json(task)).into_response())
}

async fn create_json(
    State(state): State<AppState>,
    ApiJson(input): ApiJson<CreateTask>,
) -> Result<Response, ApiError> {
    store(&state, input)
}

async fn create_form(
    State(state): State<AppState>,
    ApiForm(form): ApiForm<TaskForm>,
) -> Result<Response, ApiError> {
    store(&state, form.into())
}

async fn get_task(
    State(state): State<AppState>,
    Path(id): Path<i64>,
) -> Result<Json<Task>, ApiError> {
    let tasks = state.tasks.lock().expect("store lock is never poisoned");
    tasks
        .iter()
        .find(|t| t.id == id)
        .cloned()
        .map(Json)
        .ok_or(ApiError::NotFound)
}

fn app() -> Router {
    Router::new()
        .route("/tasks", post(create_json))
        .route("/tasks/form", post(create_form))
        .route("/tasks/{id}", get(get_task))
        .with_state(AppState::default())
}

async fn shutdown_signal() {
    let ctrl_c = async {
        tokio::signal::ctrl_c()
            .await
            .expect("install Ctrl-C handler")
    };
    #[cfg(unix)]
    let terminate = async {
        tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate())
            .expect("install SIGTERM handler")
            .recv()
            .await;
    };
    #[cfg(not(unix))]
    let terminate = std::future::pending::<()>();
    tokio::select! { _ = ctrl_c => {}, _ = terminate => {} }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let addr = std::env::var("ADDR").unwrap_or_else(|_| "127.0.0.1:8080".into());
    let listener = tokio::net::TcpListener::bind(&addr).await?;
    eprintln!("listening on http://{addr}");
    axum::serve(listener, app())
        .with_graceful_shutdown(shutdown_signal())
        .await?;
    Ok(())
}
