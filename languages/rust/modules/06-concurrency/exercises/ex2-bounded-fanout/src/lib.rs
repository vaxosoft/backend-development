//! Exercise 2: bounded fan-out with async tasks.
//!
//! Run an async function over many items with **at most `limit` in flight**.
//! `map_concurrent` keeps the input order. `map_concurrent_fail_fast` returns
//! as soon as one call fails, and the calls still in flight are cancelled simply by
//! dropping their futures (async cancellation is "stop polling").
//!
//! `futures::stream` (`iter`, `buffered`, `buffer_unordered`) is the shortest route.

use std::future::Future;

/// Results are in the same order as `items`, whatever order the calls finish in.
/// A `limit` of 0 is treated as 1.
pub async fn map_concurrent<T, R, F, Fut>(items: Vec<T>, limit: usize, f: F) -> Vec<R>
where
    F: FnMut(T) -> Fut,
    Fut: Future<Output = R>,
{
    let _ = (items, limit, f);
    todo!("stream::iter(items).map(f).buffered(limit)...")
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
    let _ = (items, limit, f);
    todo!("buffer_unordered over (index, future), then put each value back at its index")
}
