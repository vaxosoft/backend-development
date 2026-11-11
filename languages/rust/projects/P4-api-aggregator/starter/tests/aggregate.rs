//! Black-box tests: the aggregator runs on a loopback port and talks to a fake upstream.

use std::collections::HashMap;
use std::sync::Arc;
use std::sync::Mutex;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::time::{Duration, Instant};

use axum::Router;
use axum::extract::{Path, State};
use axum::http::StatusCode;
use axum::response::{IntoResponse, Response};
use axum::routing::get;
use p4_api_aggregator::{Config, build_app};
use serde_json::{Value, json};
use tokio::net::TcpListener;

#[derive(Clone, Copy)]
enum Behavior {
    Ok,
    DelayMs(u64),
    Status(u16),
    /// Fail with 500 for the first `n` attempts, then succeed.
    FailFirst(u32),
}

#[derive(Default)]
struct Upstream {
    behaviors: Mutex<HashMap<u64, Behavior>>,
    attempts: Mutex<HashMap<u64, u32>>,
    in_flight: AtomicUsize,
    peak: AtomicUsize,
    default_delay_ms: u64,
}

impl Upstream {
    fn attempts(&self, id: u64) -> u32 {
        self.attempts.lock().unwrap().get(&id).copied().unwrap_or(0)
    }
    fn peak(&self) -> usize {
        self.peak.load(Ordering::SeqCst)
    }
    fn in_flight(&self) -> usize {
        self.in_flight.load(Ordering::SeqCst)
    }
}

/// Decrements the gauge when the handler finishes *or is dropped* (connection closed).
struct InFlight<'a>(&'a Upstream);

impl<'a> InFlight<'a> {
    fn enter(up: &'a Upstream) -> Self {
        let now = up.in_flight.fetch_add(1, Ordering::SeqCst) + 1;
        up.peak.fetch_max(now, Ordering::SeqCst);
        Self(up)
    }
}

impl Drop for InFlight<'_> {
    fn drop(&mut self) {
        self.0.in_flight.fetch_sub(1, Ordering::SeqCst);
    }
}

async fn item(State(up): State<Arc<Upstream>>, Path(id): Path<u64>) -> Response {
    let _guard = InFlight::enter(&up);
    let attempt = {
        let mut attempts = up.attempts.lock().unwrap();
        let n = attempts.entry(id).or_insert(0);
        *n += 1;
        *n
    };
    let behavior = up
        .behaviors
        .lock()
        .unwrap()
        .get(&id)
        .copied()
        .unwrap_or(Behavior::Ok);
    if up.default_delay_ms > 0 {
        tokio::time::sleep(Duration::from_millis(up.default_delay_ms)).await;
    }
    match behavior {
        Behavior::Ok => {}
        Behavior::DelayMs(ms) => tokio::time::sleep(Duration::from_millis(ms)).await,
        Behavior::Status(code) => return StatusCode::from_u16(code).unwrap().into_response(),
        Behavior::FailFirst(n) if attempt <= n => {
            return StatusCode::INTERNAL_SERVER_ERROR.into_response();
        }
        Behavior::FailFirst(_) => {}
    }
    axum::Json(json!({"id": id, "name": format!("item-{id}")})).into_response()
}

async fn serve(app: Router) -> String {
    let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
    let addr = listener.local_addr().unwrap();
    tokio::spawn(async move { axum::serve(listener, app).await.unwrap() });
    format!("http://{addr}")
}

struct Harness {
    upstream: Arc<Upstream>,
    base: String,
    http: reqwest::Client,
}

impl Harness {
    async fn new(behaviors: &[(u64, Behavior)], tune: impl FnOnce(&mut Config)) -> Self {
        Self::with_delay(0, behaviors, tune).await
    }

    async fn with_delay(
        default_delay_ms: u64,
        behaviors: &[(u64, Behavior)],
        tune: impl FnOnce(&mut Config),
    ) -> Self {
        let upstream = Arc::new(Upstream {
            behaviors: Mutex::new(behaviors.iter().copied().collect()),
            default_delay_ms,
            ..Upstream::default()
        });
        let upstream_app = Router::new()
            .route("/items/{id}", get(item))
            .with_state(upstream.clone());
        let upstream_base = serve(upstream_app).await;

        let mut config = Config::new(upstream_base);
        config.backoff_base = Duration::from_millis(1);
        config.call_timeout = Duration::from_millis(500);
        tune(&mut config);
        let base = serve(build_app(config)).await;
        Self {
            upstream,
            base,
            http: reqwest::Client::new(),
        }
    }

    async fn get(&self, query: &str) -> (u16, Value) {
        let response = self
            .http
            .get(format!("{}/aggregate{query}", self.base))
            .send()
            .await
            .unwrap();
        let status = response.status().as_u16();
        (status, response.json().await.unwrap())
    }
}

fn ids_of(list: &Value) -> Vec<u64> {
    list.as_array()
        .unwrap()
        .iter()
        .map(|v| v["id"].as_u64().unwrap())
        .collect()
}

async fn wait_until(mut condition: impl FnMut() -> bool) -> bool {
    for _ in 0..100 {
        if condition() {
            return true;
        }
        tokio::time::sleep(Duration::from_millis(20)).await;
    }
    false
}

// ---- input validation ----

#[tokio::test]
async fn missing_ids_is_400_with_error_envelope() {
    let h = Harness::new(&[], |_| {}).await;
    let (status, body) = h.get("").await;
    assert_eq!(status, 400);
    assert_eq!(body["error"]["code"], "bad_request");
    assert!(body["error"]["message"].is_string());
}

#[tokio::test]
async fn empty_ids_is_400() {
    let h = Harness::new(&[], |_| {}).await;
    assert_eq!(h.get("?ids=").await.0, 400);
}

#[tokio::test]
async fn non_numeric_negative_or_blank_entries_are_400() {
    let h = Harness::new(&[], |_| {}).await;
    for bad in ["abc", "1,x", "-1", "1,,2", "1.5"] {
        let (status, _) = h.get(&format!("?ids={bad}")).await;
        assert_eq!(status, 400, "ids={bad}");
    }
    assert_eq!(h.upstream.attempts(1), 0, "nothing may reach the upstream");
}

#[tokio::test]
async fn more_than_50_ids_is_400_but_exactly_50_is_accepted() {
    let h = Harness::new(&[], |c| c.max_concurrency = 50).await;
    let list = |n: u64| (1..=n).map(|i| i.to_string()).collect::<Vec<_>>().join(",");
    assert_eq!(h.get(&format!("?ids={}", list(51))).await.0, 400);
    let (status, body) = h.get(&format!("?ids={}", list(50))).await;
    assert_eq!(status, 200);
    assert_eq!(body["results"].as_array().unwrap().len(), 50);
}

#[tokio::test]
async fn spaces_around_ids_are_tolerated() {
    let h = Harness::new(&[], |_| {}).await;
    let (status, body) = h.get("?ids=1,%202%20,3").await;
    assert_eq!(status, 200);
    assert_eq!(ids_of(&body["results"]), [1, 2, 3]);
}

// ---- aggregation ----

#[tokio::test]
async fn upstream_json_is_passed_through_as_data() {
    let h = Harness::new(&[], |_| {}).await;
    let (status, body) = h.get("?ids=7").await;
    assert_eq!(status, 200);
    assert_eq!(
        body,
        json!({
            "results": [{"id": 7, "data": {"id": 7, "name": "item-7"}}],
            "errors": []
        })
    );
}

#[tokio::test]
async fn results_keep_request_order_even_when_upstream_answers_out_of_order() {
    let behaviors = [
        (1, Behavior::DelayMs(120)),
        (2, Behavior::DelayMs(60)),
        (3, Behavior::Ok),
    ];
    let h = Harness::new(&behaviors, |c| c.max_concurrency = 3).await;
    let (_, body) = h.get("?ids=1,2,3").await;
    assert_eq!(ids_of(&body["results"]), [1, 2, 3]);
}

#[tokio::test]
async fn upstream_calls_run_concurrently() {
    let h = Harness::with_delay(80, &[], |c| c.max_concurrency = 6).await;
    let started = Instant::now();
    let (status, _) = h.get("?ids=1,2,3,4,5,6").await;
    assert_eq!(status, 200);
    assert!(h.upstream.peak() >= 2, "peak was {}", h.upstream.peak());
    assert!(
        started.elapsed() < Duration::from_millis(6 * 80),
        "calls look sequential: {:?}",
        started.elapsed()
    );
}

#[tokio::test]
async fn in_flight_upstream_calls_never_exceed_max_concurrency() {
    let h = Harness::with_delay(40, &[], |c| c.max_concurrency = 3).await;
    let (status, body) = h.get("?ids=1,2,3,4,5,6,7,8,9,10").await;
    assert_eq!(status, 200);
    assert_eq!(body["results"].as_array().unwrap().len(), 10);
    assert_eq!(h.upstream.peak(), 3);
}

#[tokio::test]
async fn max_concurrency_of_one_serialises_calls() {
    let h = Harness::with_delay(10, &[], |c| c.max_concurrency = 1).await;
    h.get("?ids=1,2,3,4").await;
    assert_eq!(h.upstream.peak(), 1);
}

// ---- failures and retries ----

#[tokio::test]
async fn partial_failure_is_200_with_results_and_errors() {
    let h = Harness::new(&[(2, Behavior::Status(404))], |_| {}).await;
    let (status, body) = h.get("?ids=1,2,3").await;
    assert_eq!(status, 200);
    assert_eq!(ids_of(&body["results"]), [1, 3]);
    assert_eq!(
        body["errors"],
        json!([{"id": 2, "error": "upstream status 404"}])
    );
}

#[tokio::test]
async fn server_errors_are_retried_until_success() {
    let h = Harness::new(&[(1, Behavior::FailFirst(2))], |_| {}).await;
    let (_, body) = h.get("?ids=1").await;
    assert_eq!(ids_of(&body["results"]), [1]);
    assert_eq!(body["errors"], json!([]));
    assert_eq!(h.upstream.attempts(1), 3);
}

#[tokio::test]
async fn retries_stop_after_two_extra_attempts() {
    let h = Harness::new(&[(1, Behavior::Status(500))], |_| {}).await;
    let (status, body) = h.get("?ids=1").await;
    assert_eq!(status, 200);
    assert_eq!(
        body["errors"],
        json!([{"id": 1, "error": "upstream status 500"}])
    );
    assert_eq!(h.upstream.attempts(1), 3);
}

#[tokio::test]
async fn max_retries_is_configurable() {
    let h = Harness::new(&[(1, Behavior::Status(503))], |c| c.max_retries = 0).await;
    h.get("?ids=1").await;
    assert_eq!(h.upstream.attempts(1), 1);
}

#[tokio::test]
async fn client_errors_are_never_retried() {
    let h = Harness::new(
        &[(1, Behavior::Status(404)), (2, Behavior::Status(400))],
        |_| {},
    )
    .await;
    h.get("?ids=1,2").await;
    assert_eq!(h.upstream.attempts(1), 1);
    assert_eq!(h.upstream.attempts(2), 1);
}

#[tokio::test]
async fn slow_upstream_hits_the_per_call_timeout_and_is_retried() {
    let h = Harness::new(&[(1, Behavior::DelayMs(2_000))], |c| {
        c.call_timeout = Duration::from_millis(40);
    })
    .await;
    let (status, body) = h.get("?ids=1").await;
    assert_eq!(status, 200);
    assert_eq!(
        body["errors"],
        json!([{"id": 1, "error": "upstream timeout"}])
    );
    assert_eq!(h.upstream.attempts(1), 3);
}

#[tokio::test]
async fn unreachable_upstream_is_reported_per_id() {
    let closed = {
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        format!("http://{}", listener.local_addr().unwrap())
    };
    let base = serve(build_app({
        let mut c = Config::new(closed);
        c.backoff_base = Duration::from_millis(1);
        c
    }))
    .await;
    let body: Value = reqwest::get(format!("{base}/aggregate?ids=1,2"))
        .await
        .unwrap()
        .json()
        .await
        .unwrap();
    assert_eq!(body["results"], json!([]));
    assert_eq!(
        body["errors"],
        json!([
            {"id": 1, "error": "upstream unreachable"},
            {"id": 2, "error": "upstream unreachable"}
        ])
    );
}

// ---- deadlines and cancellation ----

#[tokio::test]
async fn overall_deadline_returns_partial_results_and_cancels_the_rest() {
    let h = Harness::new(&[(2, Behavior::DelayMs(5_000))], |c| {
        c.call_timeout = Duration::from_secs(10);
        c.overall_timeout = Duration::from_millis(150);
    })
    .await;
    let started = Instant::now();
    let (status, body) = h.get("?ids=1,2,3").await;
    assert_eq!(status, 200);
    assert!(started.elapsed() < Duration::from_secs(2));
    assert_eq!(ids_of(&body["results"]), [1, 3]);
    assert_eq!(
        body["errors"],
        json!([{"id": 2, "error": "deadline exceeded"}])
    );
    assert!(
        wait_until(|| h.upstream.in_flight() == 0).await,
        "the slow upstream call was not cancelled"
    );
}

#[tokio::test]
async fn ids_still_queued_at_the_deadline_fail_without_reaching_the_upstream() {
    let h = Harness::with_delay(300, &[], |c| {
        c.max_concurrency = 1;
        c.overall_timeout = Duration::from_millis(100);
    })
    .await;
    let (_, body) = h.get("?ids=1,2,3").await;
    assert_eq!(body["results"], json!([]));
    assert_eq!(body["errors"].as_array().unwrap().len(), 3);
    assert_eq!(h.upstream.attempts(2), 0);
    assert_eq!(h.upstream.attempts(3), 0);
}

#[tokio::test]
async fn client_disconnect_cancels_outstanding_upstream_calls() {
    let h = Harness::new(
        &[
            (1, Behavior::DelayMs(10_000)),
            (2, Behavior::DelayMs(10_000)),
        ],
        |c| c.call_timeout = Duration::from_secs(30),
    )
    .await;
    let impatient = reqwest::Client::builder()
        .timeout(Duration::from_millis(150))
        .build()
        .unwrap();
    let outcome = impatient
        .get(format!("{}/aggregate?ids=1,2", h.base))
        .send()
        .await;
    assert!(outcome.is_err(), "the client should have given up");
    assert_eq!(h.upstream.peak(), 2, "both calls should have been started");
    assert!(
        wait_until(|| h.upstream.in_flight() == 0).await,
        "upstream calls kept running after the client left"
    );
}
