export interface MapLimitOptions {
  signal?: AbortSignal;
}

/**
 * Like `Promise.all(items.map(fn))` but with at most `limit` calls of `fn` in flight at any time.
 *
 * - Results are in input order. An empty input resolves to [].
 * - `limit` must be a positive integer, otherwise reject with RangeError.
 * - Fail fast: when a call rejects, the returned promise rejects with that error and NO new call is
 *   started afterwards (calls already in flight are left to finish).
 * - If `options.signal` is aborted (before or during the run), reject with `signal.reason` and start
 *   no new calls.
 */
export async function mapLimit<T, R>(
  _items: readonly T[],
  _limit: number,
  _fn: (item: T, index: number) => Promise<R>,
  _options: MapLimitOptions = {},
): Promise<R[]> {
  throw new Error('TODO');
}
