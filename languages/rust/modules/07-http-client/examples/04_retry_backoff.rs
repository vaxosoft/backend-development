//! Hand-written retry with exponential backoff. Retries network errors, timeouts and
//! 5xx for idempotent requests; never retries 4xx.
use std::sync::{
    Arc,
    atomic::{AtomicUsize, Ordering},
};
use std::time::Duration;

use axum::{Router, extract::State, http::StatusCode, routing::get};
use reqwest::{Client, Response};

#[derive(Clone, Copy)]
struct RetryPolicy {
    max_retries: u32,
    base: Duration,
    cap: Duration,
}

impl RetryPolicy {
    /// base * 2^retry, capped. Real services also add random jitter (see the README).
    fn delay(&self, retry: u32) -> Duration {
        self.base
            .saturating_mul(2u32.saturating_pow(retry))
            .min(self.cap)
    }
}

#[derive(Debug, thiserror::Error)]
enum FetchError {
    #[error("gave up after {attempts} attempts: {last}")]
    Exhausted { attempts: u32, last: String },
    #[error("not retryable: {0}")]
    Permanent(String),
}

fn retryable_status(response: &Response) -> bool {
    response.status().is_server_error() || response.status() == StatusCode::TOO_MANY_REQUESTS
}

async fn get_with_retry(
    client: &Client,
    url: &str,
    policy: RetryPolicy,
) -> Result<Response, FetchError> {
    let mut attempts = 0;
    loop {
        attempts += 1;
        let failure = match client.get(url).send().await {
            Ok(response) if response.status().is_success() => return Ok(response),
            Ok(response) if retryable_status(&response) => format!("status {}", response.status()),
            Ok(response) => {
                return Err(FetchError::Permanent(format!(
                    "status {}",
                    response.status()
                )));
            }
            Err(error) if error.is_connect() || error.is_timeout() => error.to_string(),
            Err(error) => return Err(FetchError::Permanent(error.to_string())),
        };
        if attempts > policy.max_retries {
            return Err(FetchError::Exhausted {
                attempts,
                last: failure,
            });
        }
        let delay = policy.delay(attempts - 1);
        println!("  attempt {attempts} failed ({failure}); retrying in {delay:?}");
        tokio::time::sleep(delay).await;
    }
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    let (base, hits) = spawn_upstream().await;
    let client = Client::builder().timeout(Duration::from_secs(1)).build()?;
    let policy = RetryPolicy {
        max_retries: 3,
        base: Duration::from_millis(5),
        cap: Duration::from_millis(15),
    };

    println!("GET /flaky (fails twice, then succeeds)");
    let response = get_with_retry(&client, &format!("{base}/flaky"), policy).await?;
    println!(
        "  -> {} after {} requests",
        response.status(),
        hits.load(Ordering::SeqCst)
    );

    println!("GET /missing (404 is permanent)");
    hits.store(0, Ordering::SeqCst);
    let error = get_with_retry(&client, &format!("{base}/missing"), policy)
        .await
        .unwrap_err();
    println!("  -> {error} after {} request", hits.load(Ordering::SeqCst));

    println!("GET /broken (always 500)");
    let error = get_with_retry(&client, &format!("{base}/broken"), policy)
        .await
        .unwrap_err();
    println!("  -> {error}");
    Ok(())
}

async fn spawn_upstream() -> (String, Arc<AtomicUsize>) {
    let hits = Arc::new(AtomicUsize::new(0));
    let app = Router::new()
        .route(
            "/flaky",
            get(|State(hits): State<Arc<AtomicUsize>>| async move {
                if hits.fetch_add(1, Ordering::SeqCst) < 2 {
                    (StatusCode::SERVICE_UNAVAILABLE, "try later")
                } else {
                    (StatusCode::OK, "ok")
                }
            }),
        )
        .route(
            "/missing",
            get(|State(hits): State<Arc<AtomicUsize>>| async move {
                hits.fetch_add(1, Ordering::SeqCst);
                StatusCode::NOT_FOUND
            }),
        )
        .route(
            "/broken",
            get(|| async { StatusCode::INTERNAL_SERVER_ERROR }),
        )
        .with_state(Arc::clone(&hits));
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0")
        .await
        .expect("bind loopback");
    let addr = listener.local_addr().expect("local addr");
    tokio::spawn(async move { axum::serve(listener, app).await });
    (format!("http://{addr}"), hits)
}
