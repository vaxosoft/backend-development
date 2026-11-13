import { setTimeout as delay } from 'node:timers/promises';

export interface RetryOptions {
  /** Extra attempts after the first one. */
  retries: number;
  /** Delay before retry number n (0-based) is `baseDelayMs * 2 ** n`. */
  baseDelayMs: number;
  /** Injectable so tests do not really wait. Defaults to a timer-based sleep. */
  sleep?: (ms: number) => Promise<void>;
  signal?: AbortSignal;
}

const IDEMPOTENT = new Set(['GET', 'HEAD', 'PUT', 'DELETE', 'OPTIONS']);

function retryAfterMs(res: Response): number | undefined {
  const raw = res.headers.get('retry-after');
  if (raw === null || !/^\d+$/.test(raw)) {
    return undefined;
  }
  return Number(raw) * 1000;
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
  url: string,
  init: RequestInit,
  options: RetryOptions,
): Promise<Response> {
  const method = (init.method ?? 'GET').toUpperCase();
  const maxRetries = IDEMPOTENT.has(method) ? options.retries : 0;
  const sleep = options.sleep ?? ((ms: number) => delay(ms, undefined, { signal: options.signal }));

  for (let attempt = 0; ; attempt++) {
    let waitMs = options.baseDelayMs * 2 ** attempt;
    try {
      const res = await fetch(url, { ...init, signal: options.signal });
      if ((res.status < 500 && res.status !== 429) || attempt >= maxRetries) {
        return res;
      }
      waitMs = retryAfterMs(res) ?? waitMs;
      await res.body?.cancel();
    } catch (err) {
      if (options.signal?.aborted || attempt >= maxRetries) {
        throw err;
      }
    }
    await sleep(waitMs);
  }
}
