//! Spans give events context. `#[instrument]` opens one per call; spawned tasks only keep
//! the span if you attach it with `.instrument(...)`.
use tracing::{Instrument, Span, info, info_span, instrument};

#[instrument(skip(title), fields(title_len = title.len()), err)]
fn validate(user_id: u64, title: &str) -> Result<(), String> {
    if title.trim().is_empty() {
        return Err("title is empty".to_owned());
    }
    info!("title is valid");
    Ok(())
}

#[instrument]
fn create_task(user_id: u64, title: &str) -> Result<u64, String> {
    validate(user_id, title)?;
    info!("storing task");
    Ok(1)
}

#[instrument(fields(task_id = tracing::field::Empty))]
async fn handle(user_id: u64) {
    // Fields declared `Empty` can be filled in later, once the value is known.
    Span::current().record("task_id", 7);
    info!("handling request");
}

#[tokio::main(flavor = "current_thread")]
async fn main() {
    tracing_subscriber::fmt()
        .with_max_level(tracing::Level::DEBUG)
        .without_time() // timestamps are off only so this lesson's output is stable
        .with_target(false)
        .with_ansi(false)
        .with_writer(std::io::stdout)
        .init();

    println!("--- nested spans, and `err` logging the Err value");
    let _ = create_task(1, "Write docs");
    let _ = create_task(2, "  ");

    println!("--- async fn with a late-recorded field");
    handle(3).await;

    println!("--- spawned task: with and without the span");
    let span = info_span!("job", job_id = 9);
    let with = tokio::spawn(async { info!("inside the span") }.instrument(span));
    let without = tokio::spawn(async { info!("span is lost") });
    with.await.unwrap();
    without.await.unwrap();
}
