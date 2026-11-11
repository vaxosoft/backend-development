//! Request validation: collect every field problem and answer 422 once.
//! The endpoint is a dry run that returns the normalised task.
//!
//! ```text
//! ADDR=127.0.0.1:20411 cargo run -p m03-rest-api-design --example validation
//! ```

use axum::routing::post;
use axum::{Json, Router};
use m03_rest_api_design::tasks::{NewTask, TaskInput, validate_task};
use m03_rest_api_design::{ApiError, ApiJson, not_found_fallback, run};

async fn validate(ApiJson(input): ApiJson<TaskInput>) -> Result<Json<serde_json::Value>, ApiError> {
    let NewTask {
        title,
        status,
        due_date,
        tags,
        owner_id,
    } = validate_task(input)?;
    Ok(Json(serde_json::json!({
        "title": title,
        "status": status,
        "due_date": due_date,
        "tags": tags,
        "owner_id": owner_id,
    })))
}

fn app() -> Router {
    Router::new()
        .route("/tasks/validate", post(validate))
        .fallback(not_found_fallback)
}

#[tokio::main]
async fn main() -> std::io::Result<()> {
    run(app()).await
}
