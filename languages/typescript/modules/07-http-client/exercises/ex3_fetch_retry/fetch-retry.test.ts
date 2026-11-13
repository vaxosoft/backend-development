import assert from 'node:assert/strict';
import { createServer, type RequestListener } from 'node:http';
import type { AddressInfo } from 'node:net';
import { afterEach, describe, test } from 'node:test';
import { fetchWithRetry } from './fetch-retry.ts';

type Script = Array<{ status: number; headers?: Record<string, string> }>;

const servers: Array<ReturnType<typeof createServer>> = [];

async function listen(handler: RequestListener): Promise<string> {
  const server = createServer(handler);
  servers.push(server);
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  return `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
}

/** Answers with the scripted responses in order; the last one repeats. */
async function scripted(script: Script): Promise<{ url: string; calls: () => number }> {
  let calls = 0;
  const url = await listen((_req, res) => {
    const step = script[Math.min(calls, script.length - 1)];
    calls++;
    res.writeHead(step?.status ?? 200, step?.headers).end(`call ${calls}`);
  });
  return { url, calls: () => calls };
}

function recorder() {
  const delays: number[] = [];
  return {
    delays,
    sleep: async (ms: number) => {
      delays.push(ms);
    },
  };
}

afterEach(() => {
  for (const server of servers.splice(0)) {
    server.closeAllConnections();
    server.close();
  }
});

describe('fetchWithRetry', () => {
  test('retries 5xx with exponential backoff until it succeeds', async () => {
    const upstream = await scripted([{ status: 503 }, { status: 502 }, { status: 200 }]);
    const { delays, sleep } = recorder();
    const res = await fetchWithRetry(upstream.url, {}, { retries: 3, baseDelayMs: 10, sleep });
    assert.equal(res.status, 200);
    assert.equal(await res.text(), 'call 3');
    assert.equal(upstream.calls(), 3);
    assert.deepEqual(delays, [10, 20]);
  });

  test('does not retry other 4xx responses', async () => {
    const upstream = await scripted([{ status: 404 }]);
    const { delays, sleep } = recorder();
    const res = await fetchWithRetry(upstream.url, {}, { retries: 3, baseDelayMs: 10, sleep });
    assert.equal(res.status, 404);
    assert.equal(upstream.calls(), 1);
    assert.deepEqual(delays, []);
  });

  test('does not retry non-idempotent methods', async () => {
    const upstream = await scripted([{ status: 500 }, { status: 200 }]);
    const { sleep } = recorder();
    const res = await fetchWithRetry(
      upstream.url,
      { method: 'POST', body: '{}' },
      { retries: 3, baseDelayMs: 10, sleep },
    );
    assert.equal(res.status, 500);
    assert.equal(upstream.calls(), 1);
  });

  test('returns the last response when retries run out', async () => {
    const upstream = await scripted([{ status: 500 }]);
    const { delays, sleep } = recorder();
    const res = await fetchWithRetry(upstream.url, {}, { retries: 2, baseDelayMs: 5, sleep });
    assert.equal(res.status, 500);
    assert.equal(upstream.calls(), 3);
    assert.deepEqual(delays, [5, 10]);
  });

  test('Retry-After (seconds) replaces the backoff delay', async () => {
    const upstream = await scripted([
      { status: 429, headers: { 'Retry-After': '2' } },
      { status: 200 },
    ]);
    const { delays, sleep } = recorder();
    const res = await fetchWithRetry(upstream.url, {}, { retries: 2, baseDelayMs: 10, sleep });
    assert.equal(res.status, 200);
    assert.deepEqual(delays, [2000]);
  });

  test('retries network errors, then rethrows the last one', async () => {
    const url = await listen(() => {});
    for (const server of servers.splice(0)) {
      server.close();
    }
    const { delays, sleep } = recorder();
    await assert.rejects(
      fetchWithRetry(url, {}, { retries: 2, baseDelayMs: 1, sleep }),
      (err: unknown) => err instanceof TypeError,
    );
    assert.deepEqual(delays, [1, 2]);
  });

  test('stops retrying once the caller aborts', async () => {
    const upstream = await scripted([{ status: 500 }]);
    const controller = new AbortController();
    const sleep = async () => {
      controller.abort();
    };
    await assert.rejects(
      fetchWithRetry(
        upstream.url,
        {},
        { retries: 5, baseDelayMs: 1, sleep, signal: controller.signal },
      ),
      (err: unknown) => err instanceof Error && err.name === 'AbortError',
    );
    assert.equal(upstream.calls(), 1);
  });
});
