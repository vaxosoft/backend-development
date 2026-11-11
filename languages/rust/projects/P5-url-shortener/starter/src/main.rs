use p5_url_shortener::{Config, build_app};
use tokio::net::TcpListener;

fn config_from_env() -> Config {
    let secret = std::env::var("JWT_SECRET").unwrap_or_else(|_| {
        eprintln!("JWT_SECRET is not set: using an insecure development secret");
        "dev-only-secret-change-me".into()
    });
    let mut config = Config::new(secret);
    if let Ok(url) = std::env::var("DATABASE_URL") {
        config.database_url = url;
    }
    if let Some(burst) = std::env::var("RATE_BURST")
        .ok()
        .and_then(|v| v.parse().ok())
    {
        config.rate_burst = burst;
    }
    if let Some(refill) = std::env::var("RATE_REFILL_PER_SEC")
        .ok()
        .and_then(|v| v.parse().ok())
    {
        config.rate_refill_per_sec = refill;
    }
    config
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let addr = std::env::var("ADDR").unwrap_or_else(|_| "127.0.0.1:8080".into());
    let app = build_app(config_from_env()).await?;
    let listener = TcpListener::bind(&addr).await?;
    eprintln!("shortener listening on http://{}", listener.local_addr()?);
    axum::serve(listener, app)
        .with_graceful_shutdown(shutdown_signal())
        .await?;
    Ok(())
}

async fn shutdown_signal() {
    let ctrl_c = async {
        let _ = tokio::signal::ctrl_c().await;
    };
    #[cfg(unix)]
    let terminate = async {
        if let Ok(mut sig) =
            tokio::signal::unix::signal(tokio::signal::unix::SignalKind::terminate())
        {
            sig.recv().await;
        }
    };
    #[cfg(not(unix))]
    let terminate = std::future::pending::<()>();
    tokio::select! {
        () = ctrl_c => {},
        () = terminate => {},
    }
}
