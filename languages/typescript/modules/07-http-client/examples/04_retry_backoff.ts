// Retry with exponential backoff: only for idempotent requests, only for failures that can heal.
import { setTimeout as sleep } from 'node:timers/promises';
import { startServer } from './_shared.ts';

interface RetryOptions {
  retries: number;
  baseDelayMs: number;
  capMs: number;
  signal?: AbortSignal;
  /** Returns a factor in [0, 1]; the default spreads clients out ("full jitter"). */
  jitter?: () => number;
  onRetry?: (info: { attempt: number; delayMs: number; reason: string }) => void;
}

const IDEMPOTENT = new Set(['GET', 'HEAD', 'PUT', 'DELETE', 'OPTIONS']);

function isRetryable(res: Response): boolean {
  return res.status >= 500 || res.status === 429;
}

async function fetchWithRetry(
  url: string,
  init: RequestInit,
  options: RetryOptions,
): Promise<Response> {
  const method = (init.method ?? 'GET').toUpperCase();
  const maxRetries = IDEMPOTENT.has(method) ? options.retries : 0;
  const jitter = options.jitter ?? Math.random;

  for (let attempt = 0; ; attempt++) {
    let reason: string;
    try {
      const res = await fetch(url, { ...init, signal: options.signal });
      if (!isRetryable(res) || attempt >= maxRetries) {
        return res;
      }
      await res.body?.cancel();
      reason = `status ${res.status}`;
    } catch (err) {
      if (options.signal?.aborted || attempt >= maxRetries) {
        throw err;
      }
      reason = 'network error';
    }
    const delayMs = Math.round(
      jitter() * Math.min(options.capMs, options.baseDelayMs * 2 ** attempt),
    );
    options.onRetry?.({ attempt: attempt + 1, delayMs, reason });
    await sleep(delayMs, undefined, { signal: options.signal });
  }
}

let calls = 0;
const server = await startServer((req, res) => {
  calls++;
  if (req.url === '/flaky' && calls <= 2) {
    res.writeHead(503).end();
  } else if (req.url === '/missing') {
    res.writeHead(404).end();
  } else if (req.url === '/always-down') {
    res.writeHead(500).end();
  } else {
    res.end('ok');
  }
});

const options = {
  retries: 3,
  baseDelayMs: 10,
  capMs: 1000,
  jitter: () => 1,
  onRetry: (info: { attempt: number; delayMs: number; reason: string }) =>
    console.log(`  retry #${info.attempt} in ${info.delayMs}ms (${info.reason})`),
};

try {
  for (const [method, path] of [
    ['GET', '/flaky'],
    ['GET', '/missing'],
    ['GET', '/always-down'],
    ['POST', '/always-down'],
  ] as const) {
    calls = 0;
    console.log(method, path);
    const res = await fetchWithRetry(server.url + path, { method }, options);
    console.log(`  final status ${res.status} after ${calls} call(s)`);
    await res.body?.cancel();
  }
} finally {
  await server.close();
}
