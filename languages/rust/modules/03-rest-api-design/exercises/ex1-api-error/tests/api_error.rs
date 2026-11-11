use axum::body::Body;
use axum::http::{Request, StatusCode, header};
use http_body_util::BodyExt;
use m03_ex1_api_error::router;
use serde_json::{Value, json};
use tower::ServiceExt;

struct Reply {
    status: StatusCode,
    content_type: Option<String>,
    retry_after: Option<String>,
    json: Value,
}

async fn send(request: Request<Body>) -> Reply {
    let response = router().oneshot(request).await.unwrap();
    let header_text = |name| {
        response
            .headers()
            .get(name)
            .map(|v| v.to_str().unwrap().to_string())
    };
    let status = response.status();
    let content_type = header_text(header::CONTENT_TYPE);
    let retry_after = header_text(header::RETRY_AFTER);
    let bytes = response.into_body().collect().await.unwrap().to_bytes();
    Reply {
        status,
        content_type,
        retry_after,
        json: serde_json::from_slice(&bytes).unwrap_or(Value::Null),
    }
}

async fn fail(kind: &str) -> Reply {
    send(
        Request::get(format!("/fail/{kind}"))
            .body(Body::empty())
            .unwrap(),
    )
    .await
}

async fn post_json(body: &str) -> Reply {
    let request = Request::post("/items")
        .header(header::CONTENT_TYPE, "application/json")
        .body(Body::from(body.to_string()))
        .unwrap();
    send(request).await
}

#[tokio::test]
async fn simple_variants_map_to_status_code_and_message() {
    let cases = [
        ("bad_request", 400, "bad_request", "nope"),
        (
            "unauthorized",
            401,
            "unauthorized",
            "authentication required",
        ),
        ("forbidden", 403, "forbidden", "not allowed"),
        ("not_found", 404, "not_found", "no such thing"),
    ];
    for (kind, status, code, message) in cases {
        let reply = fail(kind).await;
        assert_eq!(reply.status.as_u16(), status, "{kind}");
        assert_eq!(
            reply.json,
            json!({ "error": { "code": code, "message": message } }),
            "{kind}"
        );
    }
}

#[tokio::test]
async fn responses_are_json() {
    let reply = fail("forbidden").await;
    assert_eq!(reply.content_type.as_deref(), Some("application/json"));
}

#[tokio::test]
async fn validation_errors_carry_their_fields() {
    let reply = fail("validation").await;
    assert_eq!(reply.status, StatusCode::UNPROCESSABLE_ENTITY);
    assert_eq!(
        reply.json,
        json!({ "error": {
            "code": "validation_failed",
            "message": "invalid item",
            "fields": { "name": "is required" }
        } })
    );
}

#[tokio::test]
async fn rate_limited_sets_retry_after() {
    let reply = fail("rate_limited").await;
    assert_eq!(reply.status, StatusCode::TOO_MANY_REQUESTS);
    assert_eq!(reply.retry_after.as_deref(), Some("7"));
    assert_eq!(reply.json["error"]["code"], "rate_limited");
    assert_eq!(reply.json["error"]["message"], "too many requests");
}

#[tokio::test]
async fn internal_errors_never_leak_details() {
    let reply = fail("anything-else").await;
    assert_eq!(reply.status, StatusCode::INTERNAL_SERVER_ERROR);
    assert_eq!(
        reply.json,
        json!({ "error": { "code": "internal", "message": "internal server error" } })
    );
    assert!(!reply.json.to_string().contains("secret"));
}

#[tokio::test]
async fn valid_bodies_pass_through() {
    let reply = post_json(r#"{"name":"widget"}"#).await;
    assert_eq!(reply.status, StatusCode::CREATED);
    assert_eq!(reply.json, json!({ "name": "widget" }));
}

#[tokio::test]
async fn malformed_json_is_a_400_envelope() {
    let reply = post_json(r#"{"name":"#).await;
    assert_eq!(reply.status, StatusCode::BAD_REQUEST);
    assert_eq!(reply.content_type.as_deref(), Some("application/json"));
    assert_eq!(reply.json["error"]["code"], "bad_request");
    assert!(reply.json["error"].get("fields").is_none());
}

#[tokio::test]
async fn wrong_shape_is_a_422_with_a_body_field() {
    let reply = post_json(r#"{"name":5}"#).await;
    assert_eq!(reply.status, StatusCode::UNPROCESSABLE_ENTITY);
    assert_eq!(reply.json["error"]["code"], "validation_failed");
    assert!(reply.json["error"]["fields"]["body"].is_string());
}

#[tokio::test]
async fn a_missing_content_type_is_a_400_envelope() {
    let request = Request::post("/items")
        .body(Body::from(r#"{"name":"x"}"#))
        .unwrap();
    let reply = send(request).await;
    assert_eq!(reply.status, StatusCode::BAD_REQUEST);
    assert_eq!(reply.json["error"]["code"], "bad_request");
}
