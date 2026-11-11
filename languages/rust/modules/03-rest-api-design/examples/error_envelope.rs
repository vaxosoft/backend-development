//! One error envelope for everything: handler errors, bad JSON, bad path/query
//! values and unknown routes.
//!
//! ```text
//! ADDR=127.0.0.1:20410 cargo run -p m03-rest-api-design --example error_envelope
//! ```

use axum::routing::{get, post};
use axum::{Json, Router};
use m03_rest_api_design::{ApiError, ApiJson, ApiPath, not_found_fallback, run};
use serde::Deserialize;
use serde_json::{Value, json};

#[derive(Deserialize)]
struct Note {
    text: String,
}

// `ApiPath` turns "abc is not a u32" into a 400 envelope instead of plain text.
async fn get_note(ApiPath(id): ApiPath<u32>) -> Result<Json<Value>, ApiError> {
    match id {
        1 => Ok(Json(json!({ "id": 1, "text": "hello" }))),
        _ => Err(ApiError::NotFound(format!("note {id} does not exist"))),
    }
}

async fn create_note(ApiJson(note): ApiJson<Note>) -> Result<Json<Value>, ApiError> {
    if note.text.trim().is_empty() {
        return Err(ApiError::validation(
            "invalid note",
            [("text".to_string(), "must not be empty".to_string())].into(),
        ));
    }
    Ok(Json(json!({ "text": note.text.trim() })))
}

async fn conflict() -> Result<(), ApiError> {
    Err(ApiError::Conflict("slug already taken".to_string()))
}

// The detail goes to the server log; the client only sees a generic message.
async fn boom() -> Result<(), ApiError> {
    Err(ApiError::Internal(
        "connection pool exhausted at db.rs:42".to_string(),
    ))
}

fn app() -> Router {
    Router::new()
        .route("/notes/{id}", get(get_note))
        .route("/notes", post(create_note))
        .route("/conflict", get(conflict))
        .route("/boom", get(boom))
        .fallback(not_found_fallback)
}

#[tokio::main]
async fn main() -> std::io::Result<()> {
    run(app()).await
}
