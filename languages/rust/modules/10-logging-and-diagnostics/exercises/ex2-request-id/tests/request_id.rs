use axum::{Extension, Router, body::Body, http::Request, routing::get};
use m10_ex2_request_id::{IdGenerator, RequestId, is_valid_request_id, with_request_ids};
use tower::ServiceExt;

async fn whoami(Extension(id): Extension<RequestId>) -> String {
    id.0
}

fn app(ids: IdGenerator) -> Router {
    with_request_ids(Router::new().route("/whoami", get(whoami)), ids)
}

async fn call(app: &Router, header: Option<&str>) -> (String, String) {
    let mut request = Request::builder().uri("/whoami");
    if let Some(value) = header {
        request = request.header("x-request-id", value);
    }
    let response = app
        .clone()
        .oneshot(request.body(Body::empty()).unwrap())
        .await
        .unwrap();
    let header = response.headers()["x-request-id"]
        .to_str()
        .unwrap()
        .to_owned();
    let body = axum::body::to_bytes(response.into_body(), 1024)
        .await
        .unwrap();
    (header, String::from_utf8(body.to_vec()).unwrap())
}

#[test]
fn generator_counts_up_and_clones_share_the_counter() {
    let ids = IdGenerator::default();
    let clone = ids.clone();
    assert_eq!(ids.next_id(), "req-0001");
    assert_eq!(clone.next_id(), "req-0002");
    assert_eq!(ids.next_id(), "req-0003");
    assert_eq!(
        IdGenerator::default().next_id(),
        "req-0001",
        "a new generator starts over"
    );
}

#[test]
fn validation_accepts_sane_ids_only() {
    assert!(is_valid_request_id("abc-123_XYZ.v2"));
    assert!(is_valid_request_id(&"a".repeat(64)));
    for bad in ["", "has space", "tab\t", "ünïcode", &"a".repeat(65)] {
        assert!(!is_valid_request_id(bad), "{bad:?}");
    }
}

#[tokio::test]
async fn a_missing_header_gets_a_generated_id_in_header_and_extension() {
    let app = app(IdGenerator::default());
    assert_eq!(
        call(&app, None).await,
        ("req-0001".into(), "req-0001".into())
    );
    assert_eq!(
        call(&app, None).await,
        ("req-0002".into(), "req-0002".into())
    );
}

#[tokio::test]
async fn a_valid_incoming_id_is_echoed_and_does_not_consume_a_generated_one() {
    let app = app(IdGenerator::default());
    assert_eq!(
        call(&app, Some("trace-from-gateway")).await,
        ("trace-from-gateway".into(), "trace-from-gateway".into())
    );
    assert_eq!(call(&app, None).await.0, "req-0001");
}

#[tokio::test]
async fn an_invalid_incoming_id_is_replaced() {
    let app = app(IdGenerator::default());
    let too_long = "x".repeat(65);
    assert_eq!(call(&app, Some(&too_long)).await.0, "req-0001");
    assert_eq!(call(&app, Some("has space")).await.0, "req-0002");
}
