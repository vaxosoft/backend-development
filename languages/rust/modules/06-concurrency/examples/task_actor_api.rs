//! Task Tracker API with shared state done two ways: an actor that owns the tasks
//! (message passing) and an atomic request counter (lock-free shared state).
//!
//! `ADDR=127.0.0.1:8080 cargo run -p m06-concurrency --example task_actor_api`

use std::sync::Arc;
use std::sync::atomic::{AtomicU64, Ordering};

use axum::extract::{Request, State};
use axum::http::{StatusCode, header};
use axum::middleware::{self, Next};
use axum::response::{IntoResponse, Response};
use axum::routing::{get, post};
use axum::{Json, Router};
use serde::{Deserialize, Serialize};
use serde_json::json;
use tokio::sync::{mpsc, oneshot};
use tokio_util::sync::CancellationToken;

#[derive(Debug, Clone, Serialize)]
struct Task {
    id: i64,
    title: String,
    status: &'static str,
    due_date: Option<String>,
    tags: Vec<String>,
    owner_id: Option<i64>,
    created_at: &'static str,
}

#[derive(Deserialize)]
struct CreateTask {
    title: String,
}

/// Every request to the actor carries a oneshot channel to send the answer back on.
enum Command {
    Create {
        title: String,
        reply: oneshot::Sender<Task>,
    },
    List {
        reply: oneshot::Sender<Vec<Task>>,
    },
}

/// The only owner of the task list: no lock needed, commands are handled one at a time.
async fn run_actor(mut commands: mpsc::Receiver<Command>) {
    let mut tasks: Vec<Task> = Vec::new();
    while let Some(command) = commands.recv().await {
        match command {
            Command::Create { title, reply } => {
                let task = Task {
                    id: tasks.len() as i64 + 1,
                    title,
                    status: "todo",
                    due_date: None,
                    tags: vec![],
                    owner_id: None,
                    created_at: "2030-01-01T09:00:00Z", // fixed so the documented output is reproducible
                };
                tasks.push(task.clone());
                let _ = reply.send(task); // the client may have gone away; that is fine
            }
            Command::List { reply } => {
                let _ = reply.send(tasks.clone());
            }
        }
    }
}

#[derive(Clone)]
struct AppState {
    commands: mpsc::Sender<Command>,
    requests: Arc<AtomicU64>,
}

enum ApiError {
    Invalid(&'static str, &'static str),
    Unavailable,
}

impl IntoResponse for ApiError {
    fn into_response(self) -> Response {
        let (status, error) = match self {
            ApiError::Invalid(field, message) => (
                StatusCode::UNPROCESSABLE_ENTITY,
                json!({"code": "validation_failed", "message": "invalid task", "fields": {field: message}}),
            ),
            ApiError::Unavailable => (
                StatusCode::INTERNAL_SERVER_ERROR,
                json!({"code": "internal", "message": "internal error"}),
            ),
        };
        (status, Json(json!({ "error": error }))).into_response()
    }
}

/// Sends a command and waits for the reply; a dead actor becomes a 500, not a panic.
async fn ask<T>(
    state: &AppState,
    build: impl FnOnce(oneshot::Sender<T>) -> Command,
) -> Result<T, ApiError> {
    let (reply, answer) = oneshot::channel();
    state
        .commands
        .send(build(reply))
        .await
        .map_err(|_| ApiError::Unavailable)?;
    answer.await.map_err(|_| ApiError::Unavailable)
}

async fn count_requests(State(state): State<AppState>, request: Request, next: Next) -> Response {
    state.requests.fetch_add(1, Ordering::Relaxed);
    next.run(request).await
}

async fn create_task(
    State(state): State<AppState>,
    Json(input): Json<CreateTask>,
) -> Result<Response, ApiError> {
    let title = input.title.trim().to_owned();
    if title.is_empty() || title.chars().count() > 200 {
        return Err(ApiError::Invalid("title", "must be 1-200 characters"));
    }
    let task = ask(&state, |reply| Command::Create { title, reply }).await?;
    let location = [(header::LOCATION, format!("/tasks/{}", task.id))];
    Ok((StatusCode::CREATED, location, Json(task)).into_response())
}

#[derive(Serialize)]
struct TaskList {
    items: Vec<Task>,
    total: usize,
}

async fn list_tasks(State(state): State<AppState>) -> Result<Json<TaskList>, ApiError> {
    let items = ask(&state, |reply| Command::List { reply }).await?;
    // Unpaged on purpose (paging is module 03/05): just `items` and `total` of the shared list envelope.
    let total = items.len();
    Ok(Json(TaskList { items, total }))
}

async fn stats(State(state): State<AppState>) -> Result<Json<serde_json::Value>, ApiError> {
    let tasks = ask(&state, |reply| Command::List { reply }).await?.len();
    Ok(Json(
        json!({ "requests": state.requests.load(Ordering::Relaxed), "tasks": tasks }),
    ))
}

fn app(state: AppState) -> Router {
    Router::new()
        .route("/tasks", post(create_task).get(list_tasks))
        .route("/stats", get(stats))
        .layer(middleware::from_fn_with_state(
            state.clone(),
            count_requests,
        ))
        .with_state(state)
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let addr = std::env::var("ADDR").unwrap_or_else(|_| "127.0.0.1:8080".into());
    let listener = tokio::net::TcpListener::bind(&addr).await?;
    eprintln!("listening on http://{addr}");

    // Bounded: if the actor falls behind, handlers wait instead of queueing without limit.
    let (commands, receiver) = mpsc::channel(64);
    let actor = tokio::spawn(run_actor(receiver));
    let state = AppState {
        commands,
        requests: Arc::new(AtomicU64::new(0)),
    };

    // One token for the whole process: signal handlers cancel it, the server watches it.
    let shutdown = CancellationToken::new();
    tokio::spawn({
        let shutdown = shutdown.clone();
        async move {
            wait_for_signal().await;
            shutdown.cancel();
        }
    });

    axum::serve(listener, app(state))
        .with_graceful_shutdown(async move { shutdown.cancelled().await })
        .await?;

    // The router (and its senders) is gone now, so the actor's channel closes and it exits.
    actor.await?;
    eprintln!("stopped cleanly");
    Ok(())
}

async fn wait_for_signal() {
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
