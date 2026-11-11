//! Pagination, filtering and sorting over a fixed set of tasks.
//!
//! ```text
//! ADDR=127.0.0.1:20412 cargo run -p m03-rest-api-design --example pagination
//! ```

use std::sync::Arc;

use axum::extract::State;
use axum::routing::get;
use axum::{Json, Router};
use m03_rest_api_design::list::{ListQuery, Page, list_tasks};
use m03_rest_api_design::tasks::{Status, Task};
use m03_rest_api_design::{ApiError, ApiQuery, not_found_fallback, run};

fn seed() -> Vec<Task> {
    let rows = [
        (
            "Write docs",
            Status::Todo,
            Some("2030-02-01"),
            &["docs", "p1"][..],
        ),
        (
            "Fix login bug",
            Status::InProgress,
            Some("2030-01-15"),
            &["bug", "p1"],
        ),
        ("Release v1", Status::Done, None, &["release"]),
        ("Refactor router", Status::Todo, None, &["tech-debt"]),
        (
            "Update deps",
            Status::Todo,
            Some("2030-01-10"),
            &["chore", "p1"],
        ),
    ];
    rows.iter()
        .enumerate()
        .map(|(i, (title, status, due, tags))| Task {
            id: i as u32 + 1,
            title: title.to_string(),
            status: *status,
            due_date: due.map(str::to_string),
            tags: tags.iter().map(|t| t.to_string()).collect(),
            owner_id: None,
            created_at: format!("2030-01-0{}T09:00:00Z", i + 1),
        })
        .collect()
}

async fn list(
    State(tasks): State<Arc<Vec<Task>>>,
    ApiQuery(query): ApiQuery<ListQuery>,
) -> Result<Json<Page<Task>>, ApiError> {
    list_tasks(&tasks, &query).map(Json)
}

fn app() -> Router {
    Router::new()
        .route("/tasks", get(list))
        .with_state(Arc::new(seed()))
        .fallback(not_found_fallback)
}

#[tokio::main]
async fn main() -> std::io::Result<()> {
    run(app()).await
}
