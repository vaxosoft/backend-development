//! Trait-based fakes: no mocking framework, just small types that implement the trait.

use std::sync::{Arc, Mutex};

use axum::body::Body;
use axum::http::{Request, StatusCode, header};
use chrono::{DateTime, Duration, TimeZone, Utc};
use http_body_util::BodyExt;
use m12_testing::{Clock, MemoryStore, Status, StoreError, Task, TaskStore, app};
use tower::ServiceExt;

/// A clock that only moves when the test says so.
#[derive(Clone)]
struct FakeClock(Arc<Mutex<DateTime<Utc>>>);

impl FakeClock {
    fn at(t: DateTime<Utc>) -> Self {
        Self(Arc::new(Mutex::new(t)))
    }

    fn advance(&self, d: Duration) {
        *self.0.lock().unwrap() += d;
    }
}

impl Clock for FakeClock {
    fn now(&self) -> DateTime<Utc> {
        *self.0.lock().unwrap()
    }
}

/// Always fails: proves the handler maps store errors to a safe 500.
struct BrokenStore;

impl TaskStore for BrokenStore {
    fn insert(&self, _: &str, _: DateTime<Utc>) -> Result<Task, StoreError> {
        Err(StoreError("connection to 10.0.0.5:5432 refused".into()))
    }
    fn get(&self, _: u64) -> Result<Option<Task>, StoreError> {
        Err(StoreError("boom".into()))
    }
    fn list(&self, _: Option<Status>) -> Result<Vec<Task>, StoreError> {
        Err(StoreError("boom".into()))
    }
    fn delete(&self, _: u64) -> Result<bool, StoreError> {
        Err(StoreError("boom".into()))
    }
}

/// Wraps a real store and records the calls it saw (a "spy").
struct SpyStore {
    inner: MemoryStore,
    calls: Mutex<Vec<String>>,
}

impl TaskStore for SpyStore {
    fn insert(&self, title: &str, at: DateTime<Utc>) -> Result<Task, StoreError> {
        self.calls.lock().unwrap().push(format!("insert({title})"));
        self.inner.insert(title, at)
    }
    fn get(&self, id: u64) -> Result<Option<Task>, StoreError> {
        self.calls.lock().unwrap().push(format!("get({id})"));
        self.inner.get(id)
    }
    fn list(&self, status: Option<Status>) -> Result<Vec<Task>, StoreError> {
        self.calls.lock().unwrap().push("list".into());
        self.inner.list(status)
    }
    fn delete(&self, id: u64) -> Result<bool, StoreError> {
        self.calls.lock().unwrap().push(format!("delete({id})"));
        self.inner.delete(id)
    }
}

async fn post_task(router: axum::Router, title: &str) -> (StatusCode, serde_json::Value) {
    let req = Request::builder()
        .method("POST")
        .uri("/tasks")
        .header(header::CONTENT_TYPE, "application/json")
        .body(Body::from(serde_json::json!({"title": title}).to_string()))
        .unwrap();
    let resp = router.oneshot(req).await.unwrap();
    let status = resp.status();
    let bytes = resp.into_body().collect().await.unwrap().to_bytes();
    (status, serde_json::from_slice(&bytes).unwrap())
}

#[tokio::test]
async fn created_at_comes_from_the_injected_clock() {
    let clock = FakeClock::at(Utc.with_ymd_and_hms(2030, 1, 1, 9, 0, 0).unwrap());
    let router = app(Arc::new(MemoryStore::new()), Arc::new(clock.clone()));

    let (_, first) = post_task(router.clone(), "first").await;
    clock.advance(Duration::minutes(90));
    let (_, second) = post_task(router, "second").await;

    assert_eq!(first["created_at"], "2030-01-01T09:00:00Z");
    assert_eq!(second["created_at"], "2030-01-01T10:30:00Z");
}

#[tokio::test]
async fn store_failure_becomes_a_500_without_details() {
    let clock = FakeClock::at(Utc.with_ymd_and_hms(2030, 1, 1, 0, 0, 0).unwrap());
    let router = app(Arc::new(BrokenStore), Arc::new(clock));

    let (status, body) = post_task(router, "anything").await;
    assert_eq!(status, StatusCode::INTERNAL_SERVER_ERROR);
    assert_eq!(body["error"]["code"], "internal");
    assert!(!body.to_string().contains("10.0.0.5"), "leaked: {body}");
}

#[tokio::test]
async fn invalid_input_never_reaches_the_store() {
    let spy = Arc::new(SpyStore {
        inner: MemoryStore::new(),
        calls: Mutex::new(vec![]),
    });
    let clock = FakeClock::at(Utc.with_ymd_and_hms(2030, 1, 1, 0, 0, 0).unwrap());
    let router = app(spy.clone(), Arc::new(clock));

    let (status, _) = post_task(router.clone(), "   ").await;
    assert_eq!(status, StatusCode::UNPROCESSABLE_ENTITY);
    assert!(spy.calls.lock().unwrap().is_empty());

    post_task(router, "real").await;
    assert_eq!(*spy.calls.lock().unwrap(), ["insert(real)"]);
}
