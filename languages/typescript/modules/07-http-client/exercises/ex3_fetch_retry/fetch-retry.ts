export interface RetryOptions {
  /** Extra attempts after the first one. */
  retries: number;
  /** Delay before retry number n (0-based) is `baseDelayMs * 2 ** n`. */
  baseDelayMs: number;
  /** Injectable so tests do not really wait. Defaults to a timer-based sleep. */
  sleep?: (ms: number) => Promise<void>;
  signal?: AbortSignal;
}

/**
 * fetch with retries:
 * - only idempotent methods are retried (GET, HEAD, PUT, DELETE, OPTIONS; default method is GET)
 * - retry on a network error (fetch rejects) and on status 429 or >= 500; never on other 4xx
 * - a numeric `Retry-After` header (seconds) replaces the backoff delay for that retry
 * - cancel the body of a response you are about to discard
 * - when retries run out: return the last response, or rethrow the last network error
 * - if `options.signal` is aborted, stop retrying and rethrow
 */
export async function fetchWithRetry(
  _url: string,
  _init: RequestInit,
  _options: RetryOptions,
): Promise<Response> {
  throw new Error('TODO');
}
