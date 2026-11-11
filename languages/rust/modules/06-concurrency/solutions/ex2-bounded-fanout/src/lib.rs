//! Exercise 2: bounded fan-out with async tasks.
//!
//! Run an async function over many items with **at most `limit` in flight**.
//! `map_concurrent` keeps the input order. `map_concurrent_fail_fast` returns
//! as soon as one call fails, and the calls still in flight are cancelled simply by
//! dropping their futures (async cancellation is "stop polling").
//!
//! `futures::stream` (`iter`, `buffered`, `buffer_unordered`) is the shortest route.

use std::future::Future;

use futures::StreamExt;
use futures::stream;

/// Results are in the same order as `items`, whatever order the calls finish in.
/// A `limit` of 0 is treated as 1.
pub async fn map_concurrent<T, R, F, Fut>(items: Vec<T>, limit: usize, f: F) -> Vec<R>
where
    F: FnMut(T) -> Fut,
    Fut: Future<Output = R>,
{
    stream::iter(items)
        .map(f)
        .buffered(limit.max(1))
        .collect()
        .await
}

/// On success the values are in input order. On the first `Err` (by completion
/// time, not by position) return it immediately; no further calls may be started
/// and the ones in flight must be dropped.
pub async fn map_concurrent_fail_fast<T, R, E, F, Fut>(
    items: Vec<T>,
    limit: usize,
    f: F,
) -> Result<Vec<R>, E>
where
    F: Fn(T) -> Fut,
    Fut: Future<Output = Result<R, E>>,
{
    let f = &f;
    let mut slots: Vec<Option<R>> = items.iter().map(|_| None).collect();
    let mut running = stream::iter(items.into_iter().enumerate())
        .map(|(index, item)| async move { (index, f(item).await) })
        .buffer_unordered(limit.max(1));
    while let Some((index, result)) = running.next().await {
        slots[index] = Some(result?);
    }
    Ok(slots
        .into_iter()
        .map(|slot| slot.expect("every slot is filled"))
        .collect())
}
