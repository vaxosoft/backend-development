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
export async function fetchWithTimeout(
  _url: string,
  _options: TimedFetchOptions,
): Promise<Response> {
  throw new Error('TODO');
}
