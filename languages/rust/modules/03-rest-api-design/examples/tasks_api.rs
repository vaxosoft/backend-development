//! Task Tracker REST API: resources, verbs, status codes, validation, errors,
//! pagination, a `/v1` prefix and a hand-written OpenAPI document.
//!
//! ```text
//! ADDR=127.0.0.1:20414 cargo run -p m03-rest-api-design --example tasks_api
//! ```

use std::sync::{Arc, Mutex};

use axum::extract::State;
use axum::http::{StatusCode, header};
use axum::response::{IntoResponse, Response};
use axum::routing::get;
use axum::{Json, Router};
use m03_rest_api_design::list::{ListQuery, Page, list_tasks};
use m03_rest_api_design::tasks::{
    PatchInput, Task, TaskInput, TaskStore, validate_patch, validate_task,
};
use m03_rest_api_design::{ApiError, ApiJson, ApiPath, ApiQuery, not_found_fallback, run};

type Db = Arc<Mutex<TaskStore>>;

fn not_found(id: u32) -> ApiError {
    ApiError::NotFound(format!("task {id} not found"))
}

async fn create(
    State(db): State<Db>,
    ApiJson(input): ApiJson<TaskInput>,
) -> Result<Response, ApiError> {
    let new = validate_task(input)?;
    let task = db.lock().expect("store mutex poisoned").create(new);
    Ok((
        StatusCode::CREATED,
        [(header::LOCATION, format!("/v1/tasks/{}", task.id))],
        Json(task),
    )
        .into_response())
}

async fn list(
    State(db): State<Db>,
    ApiQuery(query): ApiQuery<ListQuery>,
) -> Result<Json<Page<Task>>, ApiError> {
    let tasks = db.lock().expect("store mutex poisoned").all();
    list_tasks(&tasks, &query).map(Json)
}

async fn read(State(db): State<Db>, ApiPath(id): ApiPath<u32>) -> Result<Json<Task>, ApiError> {
    let task = db.lock().expect("store mutex poisoned").get(id);
    task.map(Json).ok_or_else(|| not_found(id))
}

async fn replace(
    State(db): State<Db>,
    ApiPath(id): ApiPath<u32>,
    ApiJson(input): ApiJson<TaskInput>,
) -> Result<Json<Task>, ApiError> {
    let new = validate_task(input)?;
    let task = db.lock().expect("store mutex poisoned").replace(id, new);
    task.map(Json).ok_or_else(|| not_found(id))
}

async fn patch(
    State(db): State<Db>,
    ApiPath(id): ApiPath<u32>,
    ApiJson(input): ApiJson<PatchInput>,
) -> Result<Json<Task>, ApiError> {
    let patch = validate_patch(input)?;
    let task = db.lock().expect("store mutex poisoned").patch(id, patch);
    task.map(Json).ok_or_else(|| not_found(id))
}

async fn delete(State(db): State<Db>, ApiPath(id): ApiPath<u32>) -> Result<StatusCode, ApiError> {
    if db.lock().expect("store mutex poisoned").delete(id) {
        Ok(StatusCode::NO_CONTENT)
    } else {
        Err(not_found(id))
    }
}

async fn openapi() -> impl IntoResponse {
    (
        [(header::CONTENT_TYPE, "application/json")],
        include_str!("tasks_api.openapi.json"),
    )
}

fn app() -> Router {
    // Everything under `/v1`: a breaking change ships as `/v2` next to it.
    let v1 = Router::new().route("/tasks", get(list).post(create)).route(
        "/tasks/{id}",
        get(read).put(replace).patch(patch).delete(delete),
    );
    Router::new()
        .nest("/v1", v1)
        .route("/openapi.json", get(openapi))
        .with_state(Db::default())
        .fallback(not_found_fallback)
}

#[tokio::main]
async fn main() -> std::io::Result<()> {
    run(app()).await
}
