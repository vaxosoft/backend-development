//! Testing an outbound HTTP call against a local fake server (wiremock), never the real network.

use chrono::{TimeZone, Utc};
use m12_testing::{NotifyError, Status, Task, notify_created};
use serde_json::json;
use wiremock::matchers::{body_partial_json, header, method, path};
use wiremock::{Mock, MockServer, ResponseTemplate};

fn task() -> Task {
    Task {
        id: 7,
        title: "Ship it".into(),
        status: Status::Todo,
        created_at: Utc.with_ymd_and_hms(2030, 1, 1, 9, 0, 0).unwrap(),
    }
}

#[tokio::test]
async fn posts_the_event_as_json() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/hooks/tasks"))
        .and(header("content-type", "application/json"))
        .and(body_partial_json(
            json!({"event": "task.created", "task": {"id": 7, "title": "Ship it"}}),
        ))
        .respond_with(ResponseTemplate::new(204))
        .expect(1) // verified when `server` is dropped at the end of the test
        .mount(&server)
        .await;

    let url = format!("{}/hooks/tasks", server.uri());
    notify_created(&reqwest::Client::new(), &url, &task())
        .await
        .unwrap();
}

#[tokio::test]
async fn a_server_error_is_reported() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .respond_with(ResponseTemplate::new(503))
        .expect(1)
        .mount(&server)
        .await;

    let err = notify_created(&reqwest::Client::new(), &server.uri(), &task())
        .await
        .unwrap_err();
    assert!(
        matches!(err, NotifyError::Status(s) if s.as_u16() == 503),
        "got {err:?}"
    );
}

#[tokio::test]
async fn an_unreachable_server_is_a_transport_error() {
    // bind-and-drop: the port was free a moment ago and nothing listens on it now
    let port = std::net::TcpListener::bind("127.0.0.1:0")
        .unwrap()
        .local_addr()
        .unwrap()
        .port();
    let url = format!("http://127.0.0.1:{port}/");
    let err = notify_created(&reqwest::Client::new(), &url, &task())
        .await
        .unwrap_err();
    assert!(matches!(err, NotifyError::Transport(_)), "got {err:?}");
}
