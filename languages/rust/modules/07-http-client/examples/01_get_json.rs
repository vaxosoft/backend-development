//! One GET request: status, headers, and a JSON body decoded into a struct.
use axum::{Json, Router, routing::get};
use serde::Deserialize;
use serde_json::json;

#[derive(Debug, Deserialize)]
struct Task {
    id: u64,
    title: String,
    status: String,
    tags: Vec<String>,
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    let base = spawn_upstream().await;
    let client = reqwest::Client::new();

    let response = client
        .get(format!("{base}/tasks/1"))
        .header("Accept", "application/json")
        .send()
        .await?;
    println!("status: {}", response.status());
    println!(
        "content-type: {}",
        response.headers()["content-type"].to_str()?
    );

    // `json()` consumes the response: the body can only be read once.
    let task: Task = response.json().await?;
    println!("{task:?}");
    println!(
        "#{} {:?} is {} with {} tags",
        task.id,
        task.title,
        task.status,
        task.tags.len()
    );

    let text = client
        .get(format!("{base}/ping"))
        .send()
        .await?
        .text()
        .await?;
    println!("text body: {text}");
    Ok(())
}

/// A tiny in-process upstream so the example runs offline.
async fn spawn_upstream() -> String {
    let app = Router::new()
        .route("/ping", get(|| async { "pong" }))
        .route(
            "/tasks/1",
            get(|| async {
                Json(json!({"id": 1, "title": "Write docs", "status": "todo",
                    "due_date": null, "tags": ["docs", "p1"], "owner_id": null,
                    "created_at": "2030-01-01T09:00:00Z"}))
            }),
        );
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0")
        .await
        .expect("bind loopback");
    let addr = listener.local_addr().expect("local addr");
    tokio::spawn(async move { axum::serve(listener, app).await });
    format!("http://{addr}")
}
