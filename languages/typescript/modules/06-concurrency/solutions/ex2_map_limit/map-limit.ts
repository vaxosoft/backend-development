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
  items: readonly T[],
  limit: number,
  fn: (item: T, index: number) => Promise<R>,
  options: MapLimitOptions = {},
): Promise<R[]> {
  if (!Number.isInteger(limit) || limit < 1) {
    throw new RangeError(`limit must be a positive integer, got ${limit}`);
  }
  const { signal } = options;
  signal?.throwIfAborted();

  const results = new Array<R>(items.length);
  let next = 0;
  let stopped = false;

  const runner = async (): Promise<void> => {
    while (!stopped && next < items.length) {
      const index = next++;
      try {
        results[index] = await fn(items[index] as T, index);
      } catch (err: unknown) {
        stopped = true;
        throw err;
      }
    }
  };

  let onAbort = () => {};
  const aborted = new Promise<never>((_, reject) => {
    onAbort = () => {
      stopped = true;
      reject(signal?.reason);
    };
    signal?.addEventListener('abort', onAbort, { once: true });
  });

  try {
    const runners = Array.from({ length: Math.min(limit, items.length) }, runner);
    await Promise.race([Promise.all(runners), aborted]);
    return results;
  } finally {
    signal?.removeEventListener('abort', onAbort);
  }
}
