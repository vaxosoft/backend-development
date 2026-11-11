use axum::{Router, body::Body, http::Request};
use m10_ex3_diagnostics_router::{Metrics, build_app};
use tower::ServiceExt;

async fn get(app: &Router, uri: &str) -> (u16, Option<String>, String) {
    let response = app
        .clone()
        .oneshot(Request::builder().uri(uri).body(Body::empty()).unwrap())
        .await
        .unwrap();
    let status = response.status().as_u16();
    let content_type = response
        .headers()
        .get("content-type")
        .map(|v| v.to_str().unwrap().to_owned());
    let body = axum::body::to_bytes(response.into_body(), 4096)
        .await
        .unwrap();
    (
        status,
        content_type,
        String::from_utf8(body.to_vec()).unwrap(),
    )
}

#[tokio::test]
async fn healthz_answers_ok() {
    let app = build_app(Metrics::default());
    let (status, content_type, body) = get(&app, "/healthz").await;
    assert_eq!(status, 200);
    assert_eq!(content_type.as_deref(), Some("application/json"));
    assert_eq!(body, r#"{"status":"ok"}"#);
}

#[tokio::test]
async fn a_panic_becomes_a_500_error_envelope_without_leaking_the_message() {
    let app = build_app(Metrics::default());
    let (status, content_type, body) = get(&app, "/boom").await;

    assert_eq!(status, 500);
    assert_eq!(content_type.as_deref(), Some("application/json"));
    assert_eq!(
        body,
        r#"{"error":{"code":"internal","message":"internal server error"}}"#
    );
    assert!(!body.contains("kaboom"));
}

#[tokio::test]
async fn the_app_keeps_serving_after_a_panic() {
    let app = build_app(Metrics::default());
    assert_eq!(get(&app, "/boom").await.0, 500);
    assert_eq!(get(&app, "/healthz").await.0, 200);
    assert_eq!(get(&app, "/boom").await.0, 500);
    assert_eq!(get(&app, "/healthz").await.0, 200);
}

#[tokio::test]
async fn metrics_count_every_response_including_panics_and_unknown_routes() {
    let app = build_app(Metrics::default());
    get(&app, "/healthz").await;
    get(&app, "/healthz").await;
    get(&app, "/boom").await;
    assert_eq!(get(&app, "/nope").await.0, 404);

    let (status, content_type, body) = get(&app, "/metrics").await;
    assert_eq!(status, 200);
    assert!(content_type.unwrap().starts_with("text/plain"));
    assert_eq!(
        body,
        "http_requests_total 4\nhttp_client_errors_total 1\nhttp_server_errors_total 1\n"
    );

    // The scrape itself is counted once it has finished.
    let (_, _, body) = get(&app, "/metrics").await;
    assert!(body.starts_with("http_requests_total 5\n"), "{body}");
}

#[tokio::test]
async fn separate_metrics_instances_do_not_share_counts() {
    let (first, second) = (build_app(Metrics::default()), build_app(Metrics::default()));
    get(&first, "/healthz").await;
    let (_, _, body) = get(&second, "/metrics").await;
    assert!(body.starts_with("http_requests_total 0\n"), "{body}");
}
