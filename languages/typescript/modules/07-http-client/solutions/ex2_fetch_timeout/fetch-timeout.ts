export class RequestTimeoutError extends Error {
  readonly timeoutMs: number;

  constructor(url: string, timeoutMs: number, options?: ErrorOptions) {
    super(`GET ${url} timed out after ${timeoutMs}ms`, options);
    this.name = 'RequestTimeoutError';
    this.timeoutMs = timeoutMs;
  }
}

export interface TimedFetchOptions {
  timeoutMs: number;
  /** Caller-owned cancellation, e.g. "the user left" or "the server is shutting down". */
  signal?: AbortSignal;
  init?: RequestInit;
}

/**
 * fetch with a deadline for the whole exchange.
 * - success: resolves with the Response
 * - deadline passes: rejects with `RequestTimeoutError` (keep the original error as `cause`)
 * - `options.signal` aborts first: rejects with the original abort error, NOT a RequestTimeoutError
 * - any other failure (e.g. connection refused): rejects unchanged
 */
export async function fetchWithTimeout(url: string, options: TimedFetchOptions): Promise<Response> {
  const timeout = AbortSignal.timeout(options.timeoutMs);
  const signal = options.signal ? AbortSignal.any([options.signal, timeout]) : timeout;
  try {
    return await fetch(url, { ...options.init, signal });
  } catch (err) {
    if (timeout.aborted && !options.signal?.aborted) {
      throw new RequestTimeoutError(url, options.timeoutMs, { cause: err });
    }
    throw err;
  }
}
