//! Idempotency: `PUT`/`DELETE` are idempotent by definition, `POST` becomes safe to
//! retry with an `Idempotency-Key` header. A replay returns the stored response
//! instead of creating a second task.
//!
//! ```text
//! ADDR=127.0.0.1:20413 cargo run -p m03-rest-api-design --example idempotency
//! ```

use std::collections::HashMap;
use std::sync::{Arc, Mutex};

use axum::extract::State;
use axum::http::{HeaderMap, HeaderValue, StatusCode, header};
use axum::response::{IntoResponse, Response};
use axum::routing::post;
use axum::{Json, Router};
use m03_rest_api_design::tasks::{Task, TaskInput, TaskStore, validate_task};
use m03_rest_api_design::{ApiError, ApiJson, not_found_fallback, run};

#[derive(Default)]
struct App {
    tasks: TaskStore,
    // key -> (fingerprint of the original body, the task created for it)
    replays: HashMap<String, (String, Task)>,
}

type Shared = Arc<Mutex<App>>;

fn created(task: &Task, replayed: bool) -> Response {
    let mut response = (
        StatusCode::CREATED,
        [(header::LOCATION, format!("/tasks/{}", task.id))],
        Json(task),
    )
        .into_response();
    if replayed {
        response
            .headers_mut()
            .insert("idempotent-replayed", HeaderValue::from_static("true"));
    }
    response
}

async fn create_task(
    State(app): State<Shared>,
    headers: HeaderMap,
    ApiJson(input): ApiJson<TaskInput>,
) -> Result<Response, ApiError> {
    let new = validate_task(input)?;
    let fingerprint = format!("{new:?}");
    let key = headers
        .get("idempotency-key")
        .and_then(|v| v.to_str().ok())
        .map(str::to_string);

    let mut app = app.lock().expect("app mutex poisoned");
    let Some(key) = key else {
        let task = app.tasks.create(new);
        return Ok(created(&task, false));
    };
    if let Some((original, task)) = app.replays.get(&key) {
        // Same key with a different body is a client bug, not a retry.
        if *original != fingerprint {
            return Err(ApiError::Conflict(
                "Idempotency-Key was already used with a different request body".to_string(),
            ));
        }
        return Ok(created(task, true));
    }
    let task = app.tasks.create(new);
    app.replays.insert(key, (fingerprint, task.clone()));
    Ok(created(&task, false))
}

async fn count_tasks(State(app): State<Shared>) -> Json<serde_json::Value> {
    let app = app.lock().expect("app mutex poisoned");
    Json(serde_json::json!({ "total": app.tasks.all().len() }))
}

fn app() -> Router {
    Router::new()
        .route("/tasks", post(create_task).get(count_tasks))
        .with_state(Shared::default())
        .fallback(not_found_fallback)
}

#[tokio::main]
async fn main() -> std::io::Result<()> {
    run(app()).await
}
