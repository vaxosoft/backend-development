use axum::Router;
use axum::body::Body;
use axum::http::{Request, StatusCode};
use http_body_util::BodyExt;
use m02_ex2_request_id::router;
use tower::ServiceExt;

struct Reply {
    status: StatusCode,
    request_id: Option<String>,
    body: String,
}

async fn send(app: &Router, method: &str, uri: &str, request_id: Option<&str>) -> Reply {
    let mut builder = Request::builder().method(method).uri(uri);
    if let Some(id) = request_id {
        builder = builder.header("x-request-id", id);
    }
    let response = app
        .clone()
        .oneshot(builder.body(Body::empty()).unwrap())
        .await
        .unwrap();
    let status = response.status();
    let request_id = response
        .headers()
        .get("x-request-id")
        .map(|v| v.to_str().unwrap().to_string());
    let bytes = response.into_body().collect().await.unwrap().to_bytes();
    Reply {
        status,
        request_id,
        body: String::from_utf8(bytes.to_vec()).unwrap(),
    }
}

fn json(body: &str) -> serde_json::Value {
    serde_json::from_str(body).unwrap()
}

#[tokio::test]
async fn ids_are_generated_in_sequence() {
    let app = router();
    let first = send(&app, "GET", "/whoami", None).await;
    let second = send(&app, "GET", "/whoami", None).await;
    assert_eq!(first.request_id.as_deref(), Some("req-1"));
    assert_eq!(second.request_id.as_deref(), Some("req-2"));
}

#[tokio::test]
async fn handlers_see_the_generated_id() {
    let app = router();
    let reply = send(&app, "GET", "/whoami", None).await;
    assert_eq!(reply.body, "req-1");
}

#[tokio::test]
async fn an_incoming_id_is_echoed_and_does_not_consume_the_counter() {
    let app = router();
    let echoed = send(&app, "GET", "/whoami", Some("abc-123")).await;
    assert_eq!(echoed.request_id.as_deref(), Some("abc-123"));
    assert_eq!(echoed.body, "abc-123");
    let next = send(&app, "GET", "/whoami", None).await;
    assert_eq!(next.request_id.as_deref(), Some("req-1"));
}

#[tokio::test]
async fn an_empty_incoming_id_is_replaced() {
    let app = router();
    let reply = send(&app, "GET", "/whoami", Some("")).await;
    assert_eq!(reply.request_id.as_deref(), Some("req-1"));
}

#[tokio::test]
async fn middleware_also_covers_error_responses() {
    let app = router();
    let reply = send(&app, "GET", "/counters/missing", None).await;
    assert_eq!(reply.status, StatusCode::NOT_FOUND);
    assert_eq!(reply.request_id.as_deref(), Some("req-1"));
    let unknown_route = send(&app, "GET", "/nope", Some("zzz")).await;
    assert_eq!(unknown_route.status, StatusCode::NOT_FOUND);
    assert_eq!(unknown_route.request_id.as_deref(), Some("zzz"));
}

#[tokio::test]
async fn counters_accumulate_per_name_in_shared_state() {
    let app = router();
    let one = send(&app, "POST", "/counters/builds", None).await;
    assert_eq!(one.status, StatusCode::OK);
    assert_eq!(
        json(&one.body),
        serde_json::json!({"name": "builds", "count": 1})
    );
    send(&app, "POST", "/counters/builds", None).await;
    send(&app, "POST", "/counters/deploys", None).await;

    let builds = send(&app, "GET", "/counters/builds", None).await;
    assert_eq!(
        json(&builds.body),
        serde_json::json!({"name": "builds", "count": 2})
    );
    let deploys = send(&app, "GET", "/counters/deploys", None).await;
    assert_eq!(
        json(&deploys.body),
        serde_json::json!({"name": "deploys", "count": 1})
    );
}

#[tokio::test]
async fn each_router_owns_its_state() {
    let a = router();
    let b = router();
    send(&a, "POST", "/counters/x", None).await;
    assert_eq!(
        send(&b, "GET", "/counters/x", None).await.status,
        StatusCode::NOT_FOUND
    );
}
