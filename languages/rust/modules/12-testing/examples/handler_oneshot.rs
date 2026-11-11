//! HTTP handler tests: call the Router directly with `oneshot`, no socket involved.

use std::sync::Arc;

use axum::Router;
use axum::body::Body;
use axum::http::{Method, Request, StatusCode, header};
use http_body_util::BodyExt;
use m12_testing::{MemoryStore, SystemClock, app};
use serde_json::{Value, json};
use tower::ServiceExt;

/// A fresh router (and store) per test, so tests never share state or depend on order.
fn router() -> Router {
    app(Arc::new(MemoryStore::new()), Arc::new(SystemClock))
}

async fn send(
    router: &Router,
    method: Method,
    uri: &str,
    body: Option<&str>,
) -> (StatusCode, Option<String>, Value) {
    let mut req = Request::builder().method(method).uri(uri);
    let body = match body {
        Some(b) => {
            req = req.header(header::CONTENT_TYPE, "application/json");
            Body::from(b.to_string())
        }
        None => Body::empty(),
    };
    // oneshot consumes the service; clone the router (cheap) to reuse it across calls
    let resp = router
        .clone()
        .oneshot(req.body(body).unwrap())
        .await
        .unwrap();
    let status = resp.status();
    let location = resp
        .headers()
        .get(header::LOCATION)
        .map(|v| v.to_str().unwrap().to_string());
    let bytes = resp.into_body().collect().await.unwrap().to_bytes();
    let json = serde_json::from_slice(&bytes).unwrap_or(Value::Null);
    (status, location, json)
}

#[tokio::test]
async fn create_then_fetch() {
    let router = router();
    let (status, location, created) = send(
        &router,
        Method::POST,
        "/tasks",
        Some(r#"{"title":"  Write docs "}"#),
    )
    .await;
    assert_eq!(status, StatusCode::CREATED);
    assert_eq!(location.as_deref(), Some("/tasks/1"));
    assert_eq!(created["title"], "Write docs");
    assert_eq!(created["status"], "todo");

    let (status, _, fetched) = send(&router, Method::GET, "/tasks/1", None).await;
    assert_eq!(status, StatusCode::OK);
    assert_eq!(fetched["id"], created["id"]);
}

#[tokio::test]
async fn error_cases_use_the_envelope() {
    let router = router();
    let cases = [
        (
            Method::GET,
            "/tasks/99",
            None,
            StatusCode::NOT_FOUND,
            "not_found",
        ),
        (
            Method::POST,
            "/tasks",
            Some(r#"{"title":"   "}"#),
            StatusCode::UNPROCESSABLE_ENTITY,
            "validation_failed",
        ),
        (
            Method::POST,
            "/tasks",
            Some("{not json"),
            StatusCode::BAD_REQUEST,
            "bad_request",
        ),
        (
            Method::GET,
            "/tasks?status=blocked",
            None,
            StatusCode::BAD_REQUEST,
            "bad_request",
        ),
        (
            Method::DELETE,
            "/tasks/99",
            None,
            StatusCode::NOT_FOUND,
            "not_found",
        ),
    ];
    for (method, uri, body, status, code) in cases {
        let (got, _, json) = send(&router, method.clone(), uri, body).await;
        assert_eq!(got, status, "{method} {uri}");
        assert_eq!(json["error"]["code"], code, "{method} {uri}");
    }
}

#[tokio::test]
async fn list_filters_by_status_and_delete_removes() {
    let router = router();
    for title in ["one", "two", "three"] {
        let body = json!({"title": title}).to_string();
        send(&router, Method::POST, "/tasks", Some(&body)).await;
    }
    let (_, _, all) = send(&router, Method::GET, "/tasks", None).await;
    assert_eq!(all["total"], 3);

    let (_, _, done) = send(&router, Method::GET, "/tasks?status=done", None).await;
    assert_eq!(done["items"], json!([]));

    let (status, ..) = send(&router, Method::DELETE, "/tasks/2", None).await;
    assert_eq!(status, StatusCode::NO_CONTENT);
    let (_, _, remaining) = send(&router, Method::GET, "/tasks", None).await;
    let titles: Vec<_> = remaining["items"]
        .as_array()
        .unwrap()
        .iter()
        .map(|t| t["title"].as_str().unwrap())
        .collect();
    assert_eq!(titles, ["one", "three"]);
}
