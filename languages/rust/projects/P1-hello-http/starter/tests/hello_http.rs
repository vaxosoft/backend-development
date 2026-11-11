//! Black-box tests: every test starts the real server on an ephemeral port.

use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant};

use p1_hello_http::{Config, LogSink, build_app, run};
use reqwest::{Client, Response, StatusCode};
use serde_json::Value;
use tokio::net::TcpListener;
use tokio::sync::oneshot;
use tokio::task::JoinHandle;

struct TestServer {
    base: String,
    client: Client,
    logs: Arc<Mutex<Vec<String>>>,
    stop: Option<oneshot::Sender<()>>,
    server: JoinHandle<std::io::Result<()>>,
}

impl TestServer {
    async fn start() -> Self {
        Self::start_with(Duration::from_secs(10)).await
    }

    async fn start_with(request_timeout: Duration) -> Self {
        let logs = Arc::new(Mutex::new(Vec::new()));
        let sink_logs = Arc::clone(&logs);
        let log_sink: LogSink =
            Arc::new(move |line| sink_logs.lock().unwrap().push(line.to_owned()));
        let app = build_app(Config {
            log_sink,
            request_timeout,
        });

        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let base = format!("http://{}", listener.local_addr().unwrap());
        let (stop, stopped) = oneshot::channel::<()>();
        let server = tokio::spawn(run(listener, app, async {
            let _ = stopped.await;
        }));
        // A fresh connection per request, so idle keep-alive sockets never skew shutdown tests.
        let client = Client::builder().pool_max_idle_per_host(0).build().unwrap();
        Self {
            base,
            client,
            logs,
            stop: Some(stop),
            server,
        }
    }

    async fn get(&self, path: &str) -> Response {
        self.client
            .get(format!("{}{path}", self.base))
            .send()
            .await
            .unwrap()
    }

    fn log_lines(&self) -> Vec<String> {
        self.logs.lock().unwrap().clone()
    }

    fn begin_shutdown(&mut self) {
        let _ = self.stop.take().expect("shutdown already started").send(());
    }
}

fn header<'a>(resp: &'a Response, name: &str) -> &'a str {
    resp.headers()
        .get(name)
        .map(|v| v.to_str().unwrap())
        .unwrap_or("")
}

#[tokio::test]
async fn healthz_returns_status_ok_json() {
    let server = TestServer::start().await;
    let resp = server.get("/healthz").await;
    assert_eq!(resp.status(), StatusCode::OK);
    assert!(header(&resp, "content-type").starts_with("application/json"));
    let body: Value = resp.json().await.unwrap();
    assert_eq!(body, serde_json::json!({"status": "ok"}));
}

#[tokio::test]
async fn hello_without_name_greets_world() {
    let server = TestServer::start().await;
    let resp = server.get("/hello").await;
    assert_eq!(resp.status(), StatusCode::OK);
    assert_eq!(resp.text().await.unwrap(), "Hello, world!");
}

#[tokio::test]
async fn hello_with_name_greets_that_name() {
    let server = TestServer::start().await;
    let resp = server.get("/hello/Ferris").await;
    assert_eq!(resp.status(), StatusCode::OK);
    assert_eq!(resp.text().await.unwrap(), "Hello, Ferris!");
}

#[tokio::test]
async fn hello_is_plain_text() {
    let server = TestServer::start().await;
    let resp = server.get("/hello/Ferris").await;
    assert!(header(&resp, "content-type").starts_with("text/plain"));
}

#[tokio::test]
async fn hello_decodes_percent_encoded_names() {
    let server = TestServer::start().await;
    let resp = server.get("/hello/J%C3%B6rg%20Ann").await;
    assert_eq!(resp.text().await.unwrap(), "Hello, Jörg Ann!");
}

#[tokio::test]
async fn slow_waits_then_answers_done() {
    let server = TestServer::start().await;
    let started = Instant::now();
    let resp = server.get("/slow?ms=150").await;
    assert_eq!(resp.status(), StatusCode::OK);
    assert_eq!(resp.text().await.unwrap(), "done");
    assert!(started.elapsed() >= Duration::from_millis(140));
}

#[tokio::test]
async fn slow_without_ms_answers_immediately() {
    let server = TestServer::start().await;
    let started = Instant::now();
    let resp = server.get("/slow").await;
    assert_eq!(resp.text().await.unwrap(), "done");
    assert!(started.elapsed() < Duration::from_millis(500));
}

#[tokio::test]
async fn slow_caps_the_delay_at_two_seconds() {
    let server = TestServer::start().await;
    let started = Instant::now();
    let resp = server.get("/slow?ms=60000").await;
    assert_eq!(resp.text().await.unwrap(), "done");
    let elapsed = started.elapsed();
    assert!(elapsed >= Duration::from_millis(1900), "took {elapsed:?}");
    assert!(elapsed < Duration::from_secs(5), "took {elapsed:?}");
}

#[tokio::test]
async fn slow_with_non_numeric_ms_is_a_bad_request_envelope() {
    let server = TestServer::start().await;
    let resp = server.get("/slow?ms=abc").await;
    assert_eq!(resp.status(), StatusCode::BAD_REQUEST);
    assert!(header(&resp, "content-type").starts_with("application/json"));
    let body: Value = resp.json().await.unwrap();
    assert_eq!(body["error"]["code"], "bad_request");
}

#[tokio::test]
async fn panic_route_answers_500_with_error_envelope() {
    let server = TestServer::start().await;
    let resp = server.get("/panic").await;
    assert_eq!(resp.status(), StatusCode::INTERNAL_SERVER_ERROR);
    assert!(header(&resp, "content-type").starts_with("application/json"));
    let body: Value = resp.json().await.unwrap();
    assert_eq!(body["error"]["code"], "internal");
}

#[tokio::test]
async fn panic_message_is_not_leaked_to_the_client() {
    let server = TestServer::start().await;
    let text = server.get("/panic").await.text().await.unwrap();
    assert!(text.contains("internal"), "unexpected body: {text}");
    assert!(!text.contains("boom"), "leaked panic message: {text}");
}

#[tokio::test]
async fn server_keeps_serving_after_a_panic() {
    let server = TestServer::start().await;
    assert_eq!(
        server.get("/panic").await.status(),
        StatusCode::INTERNAL_SERVER_ERROR
    );
    assert_eq!(
        server.get("/panic").await.status(),
        StatusCode::INTERNAL_SERVER_ERROR
    );
    let resp = server.get("/healthz").await;
    assert_eq!(resp.status(), StatusCode::OK);
}

#[tokio::test]
async fn request_id_is_generated_when_missing() {
    let server = TestServer::start().await;
    let first = server.get("/healthz").await;
    let second = server.get("/healthz").await;
    let (a, b) = (
        header(&first, "x-request-id"),
        header(&second, "x-request-id"),
    );
    assert!(!a.is_empty(), "no X-Request-ID on the response");
    assert_ne!(a, b, "ids must be unique per request");
}

#[tokio::test]
async fn request_id_from_client_is_echoed() {
    let server = TestServer::start().await;
    let resp = server
        .client
        .get(format!("{}/healthz", server.base))
        .header("X-Request-ID", "abc-123")
        .send()
        .await
        .unwrap();
    assert_eq!(header(&resp, "x-request-id"), "abc-123");
}

#[tokio::test]
async fn oversized_client_request_id_is_replaced() {
    let server = TestServer::start().await;
    let long = "x".repeat(500);
    let resp = server
        .client
        .get(format!("{}/healthz", server.base))
        .header("X-Request-ID", &long)
        .send()
        .await
        .unwrap();
    let id = header(&resp, "x-request-id");
    assert!(!id.is_empty());
    assert_ne!(id, long);
}

#[tokio::test]
async fn request_id_is_present_on_panic_responses() {
    let server = TestServer::start().await;
    let resp = server.get("/panic").await;
    assert!(!header(&resp, "x-request-id").is_empty());
}

#[tokio::test]
async fn every_request_produces_exactly_one_log_line() {
    let server = TestServer::start().await;
    server.get("/healthz").await;
    server.get("/hello/a").await;
    server.get("/hello/b").await;
    assert_eq!(server.log_lines().len(), 3);
}

#[tokio::test]
async fn log_line_has_method_path_status_and_duration() {
    let server = TestServer::start().await;
    server.get("/hello/Ferris").await;
    let lines = server.log_lines();
    let line = lines.first().expect("no log line written");
    assert!(line.contains("method=GET"), "{line}");
    assert!(line.contains("path=/hello/Ferris"), "{line}");
    assert!(line.contains("status=200"), "{line}");
    assert!(line.contains("duration_ms="), "{line}");
}

#[tokio::test]
async fn log_line_carries_the_request_id() {
    let server = TestServer::start().await;
    server
        .client
        .get(format!("{}/healthz", server.base))
        .header("X-Request-ID", "trace-me")
        .send()
        .await
        .unwrap();
    let lines = server.log_lines();
    let line = lines.first().expect("no log line written");
    assert!(line.contains("request_id=trace-me"), "{line}");
}

#[tokio::test]
async fn panicking_request_is_logged_with_status_500() {
    let server = TestServer::start().await;
    server.get("/panic").await;
    let lines = server.log_lines();
    assert_eq!(lines.len(), 1);
    assert!(lines[0].contains("path=/panic"), "{}", lines[0]);
    assert!(lines[0].contains("status=500"), "{}", lines[0]);
}

#[tokio::test]
async fn requests_exceeding_the_timeout_get_408() {
    let server = TestServer::start_with(Duration::from_millis(50)).await;
    let resp = server.get("/slow?ms=1000").await;
    assert_eq!(resp.status(), StatusCode::REQUEST_TIMEOUT);
    let fast = server.get("/healthz").await;
    assert_eq!(fast.status(), StatusCode::OK);
}

#[tokio::test]
async fn shutdown_lets_in_flight_request_finish() {
    let mut server = TestServer::start().await;
    let url = format!("{}/slow?ms=600", server.base);
    let client = server.client.clone();
    let in_flight = tokio::spawn(async move { client.get(url).send().await });

    tokio::time::sleep(Duration::from_millis(200)).await;
    server.begin_shutdown();

    let resp = in_flight
        .await
        .unwrap()
        .expect("in-flight request was cut off by shutdown");
    assert_eq!(resp.status(), StatusCode::OK);
    assert_eq!(resp.text().await.unwrap(), "done");
}

#[tokio::test]
async fn run_returns_after_shutdown_once_requests_are_done() {
    let mut server = TestServer::start().await;
    server.get("/healthz").await;
    server.begin_shutdown();
    let outcome = tokio::time::timeout(Duration::from_secs(3), server.server)
        .await
        .expect("run() did not return after the shutdown signal");
    outcome.unwrap().unwrap();
}

#[tokio::test]
async fn new_connections_are_refused_after_shutdown() {
    let mut server = TestServer::start().await;
    server.begin_shutdown();
    tokio::time::timeout(Duration::from_secs(3), &mut server.server)
        .await
        .expect("run() did not return after the shutdown signal")
        .unwrap()
        .unwrap();

    let err = server
        .client
        .get(format!("{}/healthz", server.base))
        .send()
        .await
        .expect_err("server still accepts connections");
    assert!(err.is_connect(), "unexpected error: {err}");
}
