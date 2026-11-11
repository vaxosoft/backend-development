//! Configure one `Client` once: timeouts, default headers, and connection reuse.
use std::sync::{
    Arc,
    atomic::{AtomicUsize, Ordering},
};
use std::time::Duration;

use axum::{
    Router,
    routing::get,
    serve::{Listener, ListenerExt},
};
use reqwest::{Client, header};

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    let (base, connections) = spawn_upstream().await;

    let mut defaults = header::HeaderMap::new();
    defaults.insert(
        header::ACCEPT,
        header::HeaderValue::from_static("text/plain"),
    );
    let client = Client::builder()
        .connect_timeout(Duration::from_millis(200))
        .timeout(Duration::from_millis(100))
        .user_agent("task-tracker-client/1.0")
        .default_headers(defaults)
        .build()?;

    for _ in 0..5 {
        // Read the body to the end so the connection returns to the pool.
        client
            .get(format!("{base}/ping"))
            .send()
            .await?
            .text()
            .await?;
    }
    println!(
        "one client, 5 requests -> {} connection(s)",
        connections.load(Ordering::SeqCst)
    );

    connections.store(0, Ordering::SeqCst);
    for _ in 0..5 {
        Client::new()
            .get(format!("{base}/ping"))
            .send()
            .await?
            .text()
            .await?;
    }
    println!(
        "new client per request -> {} connection(s)",
        connections.load(Ordering::SeqCst)
    );

    println!(
        "echo: {}",
        client
            .get(format!("{base}/headers"))
            .send()
            .await?
            .text()
            .await?
    );

    match client.get(format!("{base}/slow")).send().await {
        Ok(response) => println!("unexpected {}", response.status()),
        Err(error) => println!("slow call: timeout={}", error.is_timeout()),
    }

    // A per-request timeout overrides the client default.
    let response = client
        .get(format!("{base}/slow"))
        .timeout(Duration::from_secs(2))
        .send()
        .await?;
    println!("slow call with 2s budget: {}", response.text().await?);
    Ok(())
}

async fn spawn_upstream() -> (String, Arc<AtomicUsize>) {
    let connections = Arc::new(AtomicUsize::new(0));
    let counter = Arc::clone(&connections);
    let app = Router::new()
        .route("/ping", get(|| async { "pong" }))
        .route(
            "/headers",
            get(|headers: axum::http::HeaderMap| async move {
                format!(
                    "accept={} user-agent={}",
                    headers["accept"].to_str().unwrap_or("?"),
                    headers["user-agent"].to_str().unwrap_or("?")
                )
            }),
        )
        .route(
            "/slow",
            get(|| async {
                tokio::time::sleep(Duration::from_millis(300)).await;
                "finally"
            }),
        );
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0")
        .await
        .expect("bind loopback")
        .tap_io(move |_| {
            counter.fetch_add(1, Ordering::SeqCst);
        });
    let addr = listener.local_addr().expect("local addr");
    tokio::spawn(async move { axum::serve(listener, app).await });
    (format!("http://{addr}"), connections)
}
