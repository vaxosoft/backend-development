//! POST JSON with a bearer token, and three ways a call can fail:
//! transport error, non-2xx status, and an undecodable body.
use axum::{
    Json, Router,
    http::{HeaderMap, StatusCode},
    routing::post,
};
use reqwest::Client;
use serde::{Deserialize, Serialize};
use serde_json::{Value, json};

#[derive(Serialize)]
struct NewTask<'a> {
    title: &'a str,
}

#[derive(Debug, Deserialize)]
struct Created {
    id: u64,
    title: String,
}

#[derive(Debug, thiserror::Error)]
enum ApiError {
    #[error("request failed: {0}")]
    Transport(#[from] reqwest::Error),
    #[error("api error {status}: {code} ({message})")]
    Api {
        status: u16,
        code: String,
        message: String,
    },
}

async fn create_task(
    client: &Client,
    base: &str,
    token: &str,
    title: &str,
) -> Result<Created, ApiError> {
    let response = client
        .post(format!("{base}/tasks"))
        .bearer_auth(token)
        .json(&NewTask { title })
        .send()
        .await?;

    let status = response.status();
    if status.is_success() {
        return Ok(response.json().await?);
    }
    // A non-2xx status is a normal `Ok(Response)`: reqwest only errors on transport problems.
    let body: Value = response.json().await.unwrap_or(Value::Null);
    Err(ApiError::Api {
        status: status.as_u16(),
        code: body["error"]["code"]
            .as_str()
            .unwrap_or("unknown")
            .to_owned(),
        message: body["error"]["message"].as_str().unwrap_or("").to_owned(),
    })
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    let base = spawn_upstream().await;
    let client = Client::new();

    let created = create_task(&client, &base, "s3cret", "Write docs").await?;
    println!("created #{} {:?}", created.id, created.title);

    for (token, title) in [("wrong", "Write docs"), ("s3cret", "")] {
        match create_task(&client, &base, token, title).await {
            Err(error) => println!("failed: {error}"),
            Ok(created) => println!("unexpected success: {created:?}"),
        }
    }

    // `error_for_status` is the short form when you do not need the body.
    let response = client
        .post(format!("{base}/tasks"))
        .json(&NewTask { title: "No token" })
        .send()
        .await?;
    match response.error_for_status() {
        Ok(_) => println!("unexpected success"),
        Err(error) => println!(
            "error_for_status: {:?} {}",
            error.status(),
            error.is_status()
        ),
    }

    // Nothing listens on port 1: a transport error, not a status.
    match client.get("http://127.0.0.1:1/").send().await {
        Ok(_) => println!("unexpected success"),
        Err(error) => println!("transport error: connect={}", error.is_connect()),
    }
    Ok(())
}

async fn spawn_upstream() -> String {
    let app = Router::new().route("/tasks", post(create_handler));
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0")
        .await
        .expect("bind loopback");
    let addr = listener.local_addr().expect("local addr");
    tokio::spawn(async move { axum::serve(listener, app).await });
    format!("http://{addr}")
}

async fn create_handler(headers: HeaderMap, Json(body): Json<Value>) -> (StatusCode, Json<Value>) {
    let authorized = headers
        .get("authorization")
        .is_some_and(|v| v == "Bearer s3cret");
    if !authorized {
        return error(
            StatusCode::UNAUTHORIZED,
            "unauthorized",
            "missing or invalid credentials",
        );
    }
    let title = body["title"].as_str().unwrap_or("");
    if title.is_empty() {
        return error(
            StatusCode::UNPROCESSABLE_ENTITY,
            "validation_failed",
            "invalid task",
        );
    }
    (StatusCode::CREATED, Json(json!({"id": 1, "title": title})))
}

fn error(status: StatusCode, code: &str, message: &str) -> (StatusCode, Json<Value>) {
    (
        status,
        Json(json!({"error": {"code": code, "message": message}})),
    )
}
