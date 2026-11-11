use m07_ex1_get_task::{FetchError, Task, get_task};
use reqwest::Client;
use serde_json::json;
use wiremock::matchers::{header, method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};

fn task_json() -> serde_json::Value {
    json!({"id": 7, "title": "Write docs", "status": "todo", "tags": ["docs"], "owner_id": null})
}

#[tokio::test]
async fn decodes_a_successful_response() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/tasks/7"))
        .and(header("accept", "application/json"))
        .respond_with(ResponseTemplate::new(200).set_body_json(task_json()))
        .expect(1)
        .mount(&server)
        .await;

    let task = get_task(&Client::new(), &server.uri(), 7).await.unwrap();
    assert_eq!(
        task,
        Task {
            id: 7,
            title: "Write docs".into(),
            status: "todo".into()
        }
    );
}

#[tokio::test]
async fn tolerates_a_trailing_slash_in_the_base_url() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .and(path("/tasks/7"))
        .respond_with(ResponseTemplate::new(200).set_body_json(task_json()))
        .mount(&server)
        .await;

    let base = format!("{}/", server.uri());
    assert_eq!(get_task(&Client::new(), &base, 7).await.unwrap().id, 7);
}

#[tokio::test]
async fn maps_404_to_not_found() {
    let server = MockServer::start().await;
    Mock::given(path("/tasks/9"))
        .respond_with(
            ResponseTemplate::new(404).set_body_json(json!({"error": {"code": "not_found"}})),
        )
        .mount(&server)
        .await;

    let error = get_task(&Client::new(), &server.uri(), 9)
        .await
        .unwrap_err();
    assert!(matches!(error, FetchError::NotFound), "got {error:?}");
}

#[tokio::test]
async fn maps_other_statuses_to_status() {
    let server = MockServer::start().await;
    Mock::given(path("/tasks/1"))
        .respond_with(ResponseTemplate::new(503))
        .mount(&server)
        .await;

    let error = get_task(&Client::new(), &server.uri(), 1)
        .await
        .unwrap_err();
    assert!(matches!(error, FetchError::Status(503)), "got {error:?}");
}

#[tokio::test]
async fn a_2xx_with_a_bad_body_is_a_decode_error_not_a_transport_error() {
    let server = MockServer::start().await;
    Mock::given(path("/tasks/1"))
        .respond_with(ResponseTemplate::new(200).set_body_string("<html>oops</html>"))
        .mount(&server)
        .await;

    let error = get_task(&Client::new(), &server.uri(), 1)
        .await
        .unwrap_err();
    assert!(matches!(error, FetchError::Decode(_)), "got {error:?}");
}

#[tokio::test]
async fn connection_failures_are_transport_errors() {
    // Bind then drop the listener: the port is free, so connecting is refused.
    let listener = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
    let base = format!("http://{}", listener.local_addr().unwrap());
    drop(listener);

    let error = get_task(&Client::new(), &base, 1).await.unwrap_err();
    assert!(matches!(error, FetchError::Transport(_)), "got {error:?}");
}
