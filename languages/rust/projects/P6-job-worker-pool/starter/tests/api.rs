use std::time::Duration;

use p6_job_worker_pool::jobs::{JobSpec, JobStatus};
use p6_job_worker_pool::pool::SubmitError;
use p6_job_worker_pool::{Config, Pool, build_app, run};
use reqwest::StatusCode;
use serde_json::{Value, json};
use tokio::net::TcpListener;

struct TestApp {
    base: String,
    pool: Pool,
    client: reqwest::Client,
}

fn test_config() -> Config {
    Config {
        workers: 2,
        queue_capacity: 16,
        max_attempts: 3,
        retry_delay: Duration::from_millis(1),
        job_timeout: Duration::from_secs(5),
        drain_timeout: Duration::from_secs(2),
    }
}

async fn start(cfg: Config) -> TestApp {
    let pool = Pool::start(cfg);
    let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
    let base = format!("http://{}", listener.local_addr().unwrap());
    let router = p6_job_worker_pool::http::router(pool.clone());
    tokio::spawn(async move { axum::serve(listener, router).await });
    TestApp {
        base,
        pool,
        client: reqwest::Client::new(),
    }
}

impl TestApp {
    async fn post(&self, body: Value) -> reqwest::Response {
        self.client
            .post(format!("{}/jobs", self.base))
            .json(&body)
            .send()
            .await
            .unwrap()
    }

    async fn submit(&self, body: Value) -> String {
        let res = self.post(body).await;
        assert_eq!(res.status(), StatusCode::ACCEPTED);
        let body: Value = res.json().await.unwrap();
        body["id"].as_str().expect("response has an id").to_owned()
    }

    async fn job(&self, id: &str) -> Value {
        self.client
            .get(format!("{}/jobs/{id}", self.base))
            .send()
            .await
            .unwrap()
            .json()
            .await
            .unwrap()
    }

    async fn delete(&self, id: &str) -> reqwest::Response {
        self.client
            .delete(format!("{}/jobs/{id}", self.base))
            .send()
            .await
            .unwrap()
    }

    async fn wait_for(&self, id: &str, status: &str) -> Value {
        let mut last = Value::Null;
        for _ in 0..600 {
            last = self.job(id).await;
            if last["status"] == status {
                return last;
            }
            tokio::time::sleep(Duration::from_millis(5)).await;
        }
        panic!("job {id} never became {status}; last view: {last}");
    }
}

fn reverse(text: &str) -> Value {
    json!({"type": "reverse", "payload": {"text": text}})
}

fn sleep_job(ms: u64) -> Value {
    json!({"type": "sleep", "payload": {"ms": ms}})
}

// ---- submitting and reading jobs -------------------------------------------

#[tokio::test]
async fn build_app_returns_a_router_that_accepts_jobs() {
    let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
    let addr = listener.local_addr().unwrap();
    tokio::spawn(axum::serve(listener, build_app(test_config())).into_future());
    let res = reqwest::Client::new()
        .post(format!("http://{addr}/jobs"))
        .json(&reverse("abc"))
        .send()
        .await
        .unwrap();
    assert_eq!(res.status(), StatusCode::ACCEPTED);
}

#[tokio::test]
async fn post_job_answers_202_with_an_id() {
    let app = start(test_config()).await;
    let res = app.post(reverse("abc")).await;
    assert_eq!(res.status(), StatusCode::ACCEPTED);
    let body: Value = res.json().await.unwrap();
    assert!(body["id"].as_str().is_some_and(|id| !id.is_empty()));
}

#[tokio::test]
async fn reverse_job_succeeds_with_reversed_text() {
    let app = start(test_config()).await;
    let id = app.submit(reverse("héllo wörld")).await;
    let job = app.wait_for(&id, "succeeded").await;
    assert_eq!(job["id"], id.as_str());
    assert_eq!(job["type"], "reverse");
    assert_eq!(job["result"], "dlröw olléh");
    assert_eq!(job["attempts"], 1);
    assert_eq!(job["error"], Value::Null);
}

#[tokio::test]
async fn sleep_job_succeeds() {
    let app = start(test_config()).await;
    let id = app.submit(sleep_job(10)).await;
    let job = app.wait_for(&id, "succeeded").await;
    assert_eq!(job["type"], "sleep");
}

#[tokio::test]
async fn unknown_job_is_404_with_error_envelope() {
    let app = start(test_config()).await;
    let res = app
        .client
        .get(format!("{}/jobs/nope", app.base))
        .send()
        .await
        .unwrap();
    assert_eq!(res.status(), StatusCode::NOT_FOUND);
    let body: Value = res.json().await.unwrap();
    assert_eq!(body["error"]["code"], "not_found");
}

#[tokio::test]
async fn malformed_json_is_400_bad_request() {
    let app = start(test_config()).await;
    let res = app
        .client
        .post(format!("{}/jobs", app.base))
        .header("content-type", "application/json")
        .body("{not json")
        .send()
        .await
        .unwrap();
    assert_eq!(res.status(), StatusCode::BAD_REQUEST);
    let body: Value = res.json().await.unwrap();
    assert_eq!(body["error"]["code"], "bad_request");
}

#[tokio::test]
async fn unknown_job_type_is_422_validation_failed() {
    let app = start(test_config()).await;
    let res = app.post(json!({"type": "explode"})).await;
    assert_eq!(res.status(), StatusCode::UNPROCESSABLE_ENTITY);
    let body: Value = res.json().await.unwrap();
    assert_eq!(body["error"]["code"], "validation_failed");
    assert!(body["error"]["fields"]["type"].is_string());
}

#[tokio::test]
async fn reverse_without_text_is_422() {
    let app = start(test_config()).await;
    let res = app.post(json!({"type": "reverse", "payload": {}})).await;
    assert_eq!(res.status(), StatusCode::UNPROCESSABLE_ENTITY);
    let body: Value = res.json().await.unwrap();
    assert!(body["error"]["fields"]["payload.text"].is_string());
}

#[tokio::test]
async fn sleep_over_5000_ms_is_422() {
    let app = start(test_config()).await;
    let res = app.post(sleep_job(5001)).await;
    assert_eq!(res.status(), StatusCode::UNPROCESSABLE_ENTITY);
    let body: Value = res.json().await.unwrap();
    assert!(body["error"]["fields"]["payload.ms"].is_string());
}

#[tokio::test]
async fn list_returns_newest_job_first() {
    let app = start(test_config()).await;
    let first = app.submit(reverse("a")).await;
    let second = app.submit(reverse("b")).await;
    let third = app.submit(reverse("c")).await;
    let body: Value = app
        .client
        .get(format!("{}/jobs", app.base))
        .send()
        .await
        .unwrap()
        .json()
        .await
        .unwrap();
    let ids: Vec<&str> = body["items"]
        .as_array()
        .unwrap()
        .iter()
        .map(|j| j["id"].as_str().unwrap())
        .collect();
    assert_eq!(ids, [third.as_str(), second.as_str(), first.as_str()]);
    assert_eq!(body["total"], 3);
}

// ---- the pool: workers, queue, retries, timeouts ---------------------------

#[tokio::test]
async fn pool_runs_as_many_jobs_at_once_as_it_has_workers() {
    let app = start(Config {
        workers: 3,
        ..test_config()
    })
    .await;
    let mut ids = Vec::new();
    for _ in 0..3 {
        ids.push(app.submit(sleep_job(5000)).await);
    }
    for id in &ids {
        app.wait_for(id, "running").await;
    }
}

#[tokio::test]
async fn extra_jobs_wait_in_the_queue_while_all_workers_are_busy() {
    let app = start(Config {
        workers: 1,
        ..test_config()
    })
    .await;
    let busy = app.submit(sleep_job(5000)).await;
    app.wait_for(&busy, "running").await;
    let waiting = app.submit(reverse("x")).await;
    let job = app.job(&waiting).await;
    assert_eq!(job["status"], "queued");
    assert_eq!(job["attempts"], 0);
}

#[tokio::test]
async fn full_queue_answers_429_with_retry_after() {
    let app = start(Config {
        workers: 1,
        queue_capacity: 1,
        ..test_config()
    })
    .await;
    let running = app.submit(sleep_job(5000)).await;
    app.wait_for(&running, "running").await;
    app.submit(sleep_job(5000)).await; // fills the queue

    let res = app.post(reverse("overflow")).await;
    assert_eq!(res.status(), StatusCode::TOO_MANY_REQUESTS);
    assert!(res.headers().contains_key("retry-after"));
    let body: Value = res.json().await.unwrap();
    assert_eq!(body["error"]["code"], "rate_limited");
    assert_eq!(app.pool.list().len(), 2, "the rejected job is not stored");
}

#[tokio::test]
async fn failing_job_is_retried_then_marked_failed() {
    let app = start(test_config()).await;
    let id = app.submit(json!({"type": "fail"})).await;
    let job = app.wait_for(&id, "failed").await;
    assert_eq!(job["attempts"], 3);
    assert_eq!(job["result"], Value::Null);
    assert!(job["error"].as_str().is_some_and(|e| !e.is_empty()));
}

#[tokio::test]
async fn max_attempts_is_configurable() {
    let app = start(Config {
        max_attempts: 2,
        ..test_config()
    })
    .await;
    let id = app.submit(json!({"type": "fail"})).await;
    let job = app.wait_for(&id, "failed").await;
    assert_eq!(job["attempts"], 2);
}

#[tokio::test]
async fn job_exceeding_the_timeout_fails_with_a_timeout_error() {
    let app = start(Config {
        max_attempts: 2,
        job_timeout: Duration::from_millis(20),
        ..test_config()
    })
    .await;
    let id = app.submit(sleep_job(3000)).await;
    let job = app.wait_for(&id, "failed").await;
    assert_eq!(job["attempts"], 2);
    assert!(job["error"].as_str().unwrap().contains("timed out"));
}

// ---- cancellation ----------------------------------------------------------

#[tokio::test]
async fn cancelling_a_running_job_answers_204_and_marks_it_cancelled() {
    let app = start(test_config()).await;
    let id = app.submit(sleep_job(5000)).await;
    app.wait_for(&id, "running").await;
    assert_eq!(app.delete(&id).await.status(), StatusCode::NO_CONTENT);
    assert_eq!(app.job(&id).await["status"], "cancelled");
}

#[tokio::test]
async fn cancelling_a_running_job_frees_its_worker() {
    let app = start(Config {
        workers: 1,
        ..test_config()
    })
    .await;
    let slow = app.submit(sleep_job(5000)).await;
    app.wait_for(&slow, "running").await;
    app.delete(&slow).await;

    let next = app.submit(reverse("ok")).await;
    app.wait_for(&next, "succeeded").await;
}

#[tokio::test]
async fn cancelled_queued_job_never_runs() {
    let app = start(Config {
        workers: 1,
        ..test_config()
    })
    .await;
    let blocker = app.submit(sleep_job(5000)).await;
    app.wait_for(&blocker, "running").await;
    let queued = app.submit(reverse("never")).await;
    assert_eq!(app.delete(&queued).await.status(), StatusCode::NO_CONTENT);

    app.delete(&blocker).await;
    let after = app.submit(reverse("after")).await;
    app.wait_for(&after, "succeeded").await;

    let job = app.job(&queued).await;
    assert_eq!(job["status"], "cancelled");
    assert_eq!(job["attempts"], 0);
}

#[tokio::test]
async fn cancelling_a_finished_job_is_409() {
    let app = start(test_config()).await;
    let id = app.submit(reverse("done")).await;
    app.wait_for(&id, "succeeded").await;
    let res = app.delete(&id).await;
    assert_eq!(res.status(), StatusCode::CONFLICT);
    let body: Value = res.json().await.unwrap();
    assert_eq!(body["error"]["code"], "conflict");
}

#[tokio::test]
async fn cancelling_an_unknown_job_is_404() {
    let app = start(test_config()).await;
    assert_eq!(app.delete("nope").await.status(), StatusCode::NOT_FOUND);
}

// ---- graceful shutdown -----------------------------------------------------

#[tokio::test]
async fn drain_lets_queued_jobs_finish() {
    let pool = Pool::start(Config {
        workers: 1,
        ..test_config()
    });
    let spec = JobSpec::Sleep { ms: 30 };
    let ids: Vec<String> = (0..3).map(|_| pool.submit(spec.clone()).unwrap()).collect();
    pool.drain().await;
    for id in ids {
        assert_eq!(pool.get(&id).unwrap().result.as_deref(), Some("slept 30ms"));
    }
}

#[tokio::test]
async fn drain_closes_the_queue_for_new_jobs() {
    let pool = Pool::start(test_config());
    pool.drain().await;
    let spec = JobSpec::Fail;
    assert_eq!(pool.submit(spec), Err(SubmitError::ShuttingDown));
}

#[tokio::test]
async fn drain_timeout_cancels_jobs_that_do_not_finish() {
    let app = start(Config {
        drain_timeout: Duration::from_millis(50),
        ..test_config()
    })
    .await;
    let id = app.submit(sleep_job(5000)).await;
    app.wait_for(&id, "running").await;

    tokio::time::timeout(Duration::from_secs(3), app.pool.drain())
        .await
        .expect("drain must return after the drain timeout");
    assert_eq!(app.pool.get(&id).unwrap().status, JobStatus::Cancelled);
}

#[tokio::test]
async fn run_finishes_running_jobs_then_stops_accepting_connections() {
    let pool = Pool::start(test_config());
    let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
    let addr = listener.local_addr().unwrap();
    let (stop, stopped) = tokio::sync::oneshot::channel::<()>();
    let server = tokio::spawn(run(listener, pool.clone(), async {
        let _ = stopped.await;
    }));

    let client = reqwest::Client::new();
    let res = client
        .post(format!("http://{addr}/jobs"))
        .json(&sleep_job(150))
        .send()
        .await
        .unwrap();
    assert_eq!(res.status(), StatusCode::ACCEPTED);
    let id = res.json::<Value>().await.unwrap()["id"]
        .as_str()
        .unwrap()
        .to_owned();
    while pool.get(&id).unwrap().attempts == 0 {
        tokio::time::sleep(Duration::from_millis(2)).await;
    }

    stop.send(()).unwrap();
    tokio::time::timeout(Duration::from_secs(3), server)
        .await
        .expect("run must return once the job finished")
        .unwrap()
        .unwrap();

    let job = pool.get(&id).unwrap();
    assert_eq!(job.result.as_deref(), Some("slept 150ms"));
    assert!(tokio::net::TcpStream::connect(addr).await.is_err());
}
