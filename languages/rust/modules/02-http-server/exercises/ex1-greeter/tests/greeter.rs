use axum::body::Body;
use axum::http::{Request, StatusCode};
use http_body_util::BodyExt;
use m02_ex1_greeter::router;
use tower::ServiceExt;

async fn get(uri: &str) -> (StatusCode, String) {
    let request = Request::get(uri).body(Body::empty()).unwrap();
    let response = router().oneshot(request).await.unwrap();
    let status = response.status();
    let bytes = response.into_body().collect().await.unwrap().to_bytes();
    (status, String::from_utf8(bytes.to_vec()).unwrap())
}

#[tokio::test]
async fn hello_without_a_name() {
    assert_eq!(
        get("/hello").await,
        (StatusCode::OK, "Hello, world!".into())
    );
}

#[tokio::test]
async fn hello_with_a_path_parameter() {
    assert_eq!(
        get("/hello/Ada").await,
        (StatusCode::OK, "Hello, Ada!".into())
    );
}

#[tokio::test]
async fn greeting_comes_from_the_query_string() {
    assert_eq!(
        get("/hello/Ada?greeting=Hi").await,
        (StatusCode::OK, "Hi, Ada!".into())
    );
}

#[tokio::test]
async fn add_sums_query_numbers() {
    assert_eq!(get("/add?a=2&b=3").await, (StatusCode::OK, "5".into()));
    assert_eq!(get("/add?a=-4&b=1").await, (StatusCode::OK, "-3".into()));
}

#[tokio::test]
async fn add_rejects_missing_or_invalid_values_with_400() {
    assert_eq!(get("/add?a=1").await.0, StatusCode::BAD_REQUEST);
    assert_eq!(get("/add?a=1&b=two").await.0, StatusCode::BAD_REQUEST);
}

#[tokio::test]
async fn two_path_parameters_become_json() {
    let (status, body) = get("/users/7/tasks/42").await;
    assert_eq!(status, StatusCode::OK);
    let json: serde_json::Value = serde_json::from_str(&body).unwrap();
    assert_eq!(json, serde_json::json!({ "user_id": 7, "task_id": 42 }));
}

#[tokio::test]
async fn non_numeric_path_parameters_are_400() {
    assert_eq!(get("/users/x/tasks/1").await.0, StatusCode::BAD_REQUEST);
}

#[tokio::test]
async fn unknown_paths_use_the_fallback() {
    assert_eq!(
        get("/nope").await,
        (StatusCode::NOT_FOUND, "not found".into())
    );
}
