import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { after, before, describe, test } from 'node:test';
import { fetchWithTimeout, RequestTimeoutError } from './fetch-timeout.ts';

describe('fetchWithTimeout', () => {
  let baseUrl = '';
  const server = createServer((req, res) => {
    if (req.url === '/fast') {
      res.end('hello');
    }
    // every other path never answers
  });

  before(async () => {
    await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
    baseUrl = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  });
  after(() => {
    server.closeAllConnections();
    server.close();
  });

  test('resolves with the response when the server answers in time', async () => {
    const res = await fetchWithTimeout(`${baseUrl}/fast`, { timeoutMs: 2000 });
    assert.equal(await res.text(), 'hello');
  });

  test('passes request init through', async () => {
    const res = await fetchWithTimeout(`${baseUrl}/fast`, {
      timeoutMs: 2000,
      init: { method: 'POST', body: 'x' },
    });
    assert.equal(res.status, 200);
  });

  test('rejects with RequestTimeoutError when the server hangs', async () => {
    const started = Date.now();
    await assert.rejects(fetchWithTimeout(`${baseUrl}/hang`, { timeoutMs: 40 }), (err: unknown) => {
      assert.ok(err instanceof RequestTimeoutError);
      assert.equal(err.timeoutMs, 40);
      assert.ok(err.cause instanceof Error);
      return true;
    });
    assert.ok(Date.now() - started < 1500);
  });

  test('a caller abort is reported as an abort, not as a timeout', async () => {
    const controller = new AbortController();
    setTimeout(() => controller.abort(), 10);
    await assert.rejects(
      fetchWithTimeout(`${baseUrl}/hang`, { timeoutMs: 5000, signal: controller.signal }),
      (err: unknown) => {
        assert.ok(!(err instanceof RequestTimeoutError));
        assert.ok(err instanceof Error);
        assert.equal(err.name, 'AbortError');
        return true;
      },
    );
  });

  test('an already aborted signal rejects immediately', async () => {
    await assert.rejects(
      fetchWithTimeout(`${baseUrl}/fast`, { timeoutMs: 5000, signal: AbortSignal.abort() }),
      (err: unknown) => err instanceof Error && err.name === 'AbortError',
    );
  });

  test('other failures are not turned into timeouts', async () => {
    await assert.rejects(
      fetchWithTimeout('http://127.0.0.1:1/', { timeoutMs: 5000 }),
      (err: unknown) => err instanceof TypeError,
    );
  });
});
