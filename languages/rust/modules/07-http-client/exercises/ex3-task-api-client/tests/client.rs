use std::time::Duration;

use m07_ex3_task_api_client::{ApiError, Status, TaskApiClient};
use serde_json::{Value, json};
use wiremock::matchers::{body_json, header, method, path, query_param};
use wiremock::{Mock, MockServer, ResponseTemplate};

const TOKEN: &str = "s3cret";

fn client(server: &MockServer) -> TaskApiClient {
    TaskApiClient::new(&server.uri(), TOKEN, Duration::from_secs(2)).unwrap()
}

fn task(id: u64) -> Value {
    json!({"id": id, "title": format!("Task {id}"), "status": "todo", "due_date": null, "tags": [], "owner_id": null})
}

fn page(items: Vec<Value>, total: usize, limit: usize, offset: usize) -> ResponseTemplate {
    ResponseTemplate::new(200)
        .set_body_json(json!({"items": items, "total": total, "limit": limit, "offset": offset}))
}

#[tokio::test]
async fn create_task_sends_bearer_token_and_json_body() {
    let server = MockServer::start().await;
    Mock::given(method("POST"))
        .and(path("/tasks"))
        .and(header("authorization", "Bearer s3cret"))
        .and(header("user-agent", "task-tracker-client/1.0"))
        .and(body_json(json!({"title": "Ship it", "tags": ["release"]})))
        .respond_with(ResponseTemplate::new(201).set_body_json(
            json!({"id": 5, "title": "Ship it", "status": "todo", "tags": ["release"]}),
        ))
        .expect(1)
        .mount(&server)
        .await;

    let created = client(&server)
        .create_task("Ship it", &["release".to_string()])
        .await
        .unwrap();
    assert_eq!(
        (created.id, created.status, created.tags),
        (5, Status::Todo, vec!["release".to_string()])
    );
}

#[tokio::test]
async fn error_envelope_becomes_an_api_error() {
    let server = MockServer::start().await;
    let envelope = json!({"error": {"code": "validation_failed", "message": "invalid task", "fields": {"title": "required"}}});
    Mock::given(method("POST"))
        .respond_with(ResponseTemplate::new(422).set_body_json(envelope))
        .mount(&server)
        .await;

    let error = client(&server).create_task("", &[]).await.unwrap_err();
    match error {
        ApiError::Api {
            status,
            code,
            message,
        } => {
            assert_eq!(
                (status, code.as_str(), message.as_str()),
                (422, "validation_failed", "invalid task")
            );
        }
        other => panic!("expected ApiError::Api, got {other:?}"),
    }
}

#[tokio::test]
async fn a_non_json_error_body_still_reports_the_status() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .respond_with(ResponseTemplate::new(502).set_body_string("<html>bad gateway</html>"))
        .mount(&server)
        .await;

    let error = client(&server).list_all().await.unwrap_err();
    assert!(
        matches!(&error, ApiError::Api { status: 502, code, .. } if code == "unknown"),
        "got {error:?}"
    );
}

#[tokio::test]
async fn list_all_follows_pages_until_total() {
    let server = MockServer::start().await;
    for (offset, items) in [
        (0, vec![task(1), task(2)]),
        (2, vec![task(3), task(4)]),
        (4, vec![task(5)]),
    ] {
        Mock::given(method("GET"))
            .and(path("/tasks"))
            .and(header("authorization", "Bearer s3cret"))
            .and(query_param("limit", "2"))
            .and(query_param("offset", offset.to_string()))
            .respond_with(page(items, 5, 2, offset))
            .expect(1)
            .mount(&server)
            .await;
    }

    let tasks = client(&server).with_page_size(2).list_all().await.unwrap();
    let ids: Vec<u64> = tasks.iter().map(|t| t.id).collect();
    assert_eq!(ids, [1, 2, 3, 4, 5]);
}

#[tokio::test]
async fn list_all_stops_on_an_empty_page() {
    let server = MockServer::start().await;
    // The server claims 10 tasks but has none: the client must not loop forever.
    Mock::given(method("GET"))
        .respond_with(page(vec![], 10, 20, 0))
        .expect(1)
        .mount(&server)
        .await;

    assert!(client(&server).list_all().await.unwrap().is_empty());
}

#[tokio::test]
async fn slow_servers_produce_a_timeout_error() {
    let server = MockServer::start().await;
    Mock::given(method("GET"))
        .respond_with(page(vec![], 0, 20, 0).set_delay(Duration::from_millis(300)))
        .mount(&server)
        .await;

    let client = TaskApiClient::new(&server.uri(), TOKEN, Duration::from_millis(30)).unwrap();
    let error = client.list_all().await.unwrap_err();
    assert!(matches!(error, ApiError::Timeout), "got {error:?}");
}

#[tokio::test]
async fn unreachable_servers_produce_a_transport_error() {
    let listener = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
    let base = format!("http://{}", listener.local_addr().unwrap());
    drop(listener);

    let error = TaskApiClient::new(&base, TOKEN, Duration::from_secs(1))
        .unwrap()
        .list_all()
        .await
        .unwrap_err();
    assert!(matches!(error, ApiError::Transport(_)), "got {error:?}");
}
