use std::sync::Arc;

use p7_tested_service_docker_ci::probe::probe;
use p7_tested_service_docker_ci::tasks::MemoryStore;
use p7_tested_service_docker_ci::{AppState, build_app};
use reqwest::StatusCode;
use serde_json::{Value, json};
use tokio::net::TcpListener;

struct TestServer {
    base: String,
    state: AppState,
    client: reqwest::Client,
}

async fn start() -> TestServer {
    let state = AppState::new(Arc::new(MemoryStore::default()));
    let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
    let base = format!("http://{}", listener.local_addr().unwrap());
    tokio::spawn(axum::serve(listener, build_app(state.clone())).into_future());
    TestServer {
        base,
        state,
        client: reqwest::Client::new(),
    }
}

impl TestServer {
    async fn get(&self, path: &str) -> reqwest::Response {
        self.client
            .get(format!("{}{path}", self.base))
            .send()
            .await
            .unwrap()
    }

    async fn post_task(&self, body: Value) -> reqwest::Response {
        self.client
            .post(format!("{}/tasks", self.base))
            .json(&body)
            .send()
            .await
            .unwrap()
    }
}

// ---- tasks (given; these pass from the start) ------------------------------

#[tokio::test]
async fn created_task_is_returned_by_get_and_list() {
    let server = start().await;
    let res = server.post_task(json!({"title": "  Write docs  "})).await;
    assert_eq!(res.status(), StatusCode::CREATED);
    assert_eq!(res.headers()["location"], "/tasks/1");
    let task: Value = res.json().await.unwrap();
    assert_eq!(
        task,
        json!({"id": 1, "title": "Write docs", "status": "todo"})
    );

    let fetched: Value = server.get("/tasks/1").await.json().await.unwrap();
    assert_eq!(fetched, task);
    let list: Value = server.get("/tasks").await.json().await.unwrap();
    assert_eq!(list, json!({"items": [task], "total": 1}));
}

#[tokio::test]
async fn unknown_task_is_404_with_error_envelope() {
    let server = start().await;
    let res = server.get("/tasks/99").await;
    assert_eq!(res.status(), StatusCode::NOT_FOUND);
    let body: Value = res.json().await.unwrap();
    assert_eq!(body["error"]["code"], "not_found");
}

#[tokio::test]
async fn empty_title_is_422_with_field_message() {
    let server = start().await;
    let res = server.post_task(json!({"title": "   "})).await;
    assert_eq!(res.status(), StatusCode::UNPROCESSABLE_ENTITY);
    let body: Value = res.json().await.unwrap();
    assert_eq!(body["error"]["code"], "validation_failed");
    assert_eq!(body["error"]["fields"]["title"], "must be 1-200 characters");
}

#[tokio::test]
async fn healthz_always_answers_ok() {
    let server = start().await;
    let res = server.get("/healthz").await;
    assert_eq!(res.status(), StatusCode::OK);
    assert_eq!(res.json::<Value>().await.unwrap(), json!({"status": "ok"}));
}

#[tokio::test]
async fn probe_succeeds_against_a_running_server() {
    let server = start().await;
    let addr = server.base.trim_start_matches("http://").to_owned();
    let healthy = tokio::task::spawn_blocking(move || probe(&addr))
        .await
        .unwrap();
    assert!(healthy);
}

// ---- readiness and metrics (your work) -------------------------------------

#[tokio::test]
async fn readyz_is_503_until_the_app_is_ready() {
    let server = start().await;
    let res = server.get("/readyz").await;
    assert_eq!(res.status(), StatusCode::SERVICE_UNAVAILABLE);
    assert_eq!(
        res.json::<Value>().await.unwrap(),
        json!({"status": "not_ready"})
    );
}

#[tokio::test]
async fn readyz_is_200_once_the_app_is_ready() {
    let server = start().await;
    server.state.set_ready(true);
    let res = server.get("/readyz").await;
    assert_eq!(res.status(), StatusCode::OK);
    assert_eq!(
        res.json::<Value>().await.unwrap(),
        json!({"status": "ready"})
    );
}

#[tokio::test]
async fn metrics_is_plain_text_in_the_expected_format() {
    let server = start().await;
    let res = server.get("/metrics").await;
    assert_eq!(res.status(), StatusCode::OK);
    let content_type = res.headers()["content-type"].to_str().unwrap().to_owned();
    assert!(content_type.starts_with("text/plain"), "{content_type}");
    assert_eq!(res.text().await.unwrap(), "requests_total 1\n");
}

#[tokio::test]
async fn metrics_counts_every_request_including_its_own() {
    let server = start().await;
    server.get("/healthz").await;
    server.get("/tasks/1").await; // a 404 still counts
    server.post_task(json!({"title": "x"})).await;
    assert_eq!(
        server.get("/metrics").await.text().await.unwrap(),
        "requests_total 4\n"
    );
}
