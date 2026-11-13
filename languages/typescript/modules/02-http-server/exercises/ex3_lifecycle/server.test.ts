import assert from 'node:assert/strict';
import { Agent, get, type IncomingMessage, type ServerResponse } from 'node:http';
import { test } from 'node:test';
import { startServer } from './server.ts';

function deferred<T = void>(): { promise: Promise<T>; resolve: (value: T) => void } {
  let resolve: (value: T) => void = () => {};
  const promise = new Promise<T>((r) => {
    resolve = r;
  });
  return { promise, resolve };
}

function request(
  port: number,
  path: string,
  agent?: Agent,
): Promise<{ status: number; body: string }> {
  return new Promise((resolve, reject) => {
    const req = get({ host: '127.0.0.1', port, path, agent: agent ?? false }, (res) => {
      let body = '';
      res.setEncoding('utf8');
      res.on('data', (chunk: string) => {
        body += chunk;
      });
      res.on('end', () => resolve({ status: res.statusCode ?? 0, body }));
      res.on('error', reject);
    });
    req.on('error', reject);
  });
}

const quick = (_req: IncomingMessage, res: ServerResponse): void => {
  res.end('ok');
};

test('listens on a free port and serves requests', async (t) => {
  const running = await startServer(quick);
  t.after(() => running.shutdown());
  assert.ok(running.port > 0);
  assert.deepEqual(await request(running.port, '/'), { status: 200, body: 'ok' });
});

test('applies default and custom timeouts', async (t) => {
  const defaults = await startServer(quick);
  t.after(() => defaults.shutdown());
  assert.equal(defaults.server.requestTimeout, 30_000);
  assert.equal(defaults.server.headersTimeout, 10_000);
  assert.equal(defaults.server.keepAliveTimeout, 5_000);

  const custom = await startServer(quick, {
    requestTimeoutMs: 2_000,
    headersTimeoutMs: 1_000,
    keepAliveTimeoutMs: 500,
  });
  t.after(() => custom.shutdown());
  assert.equal(custom.server.requestTimeout, 2_000);
  assert.equal(custom.server.headersTimeout, 1_000);
  assert.equal(custom.server.keepAliveTimeout, 500);
});

test('rejects when the port is already taken', async (t) => {
  const first = await startServer(quick);
  t.after(() => first.shutdown());
  await assert.rejects(startServer(quick, { port: first.port }), { code: 'EADDRINUSE' });
});

test('shutdown lets an in-flight request finish and refuses new connections', async () => {
  const entered = deferred();
  const release = deferred();
  const running = await startServer((req, res) => {
    if (req.url !== '/slow') return quick(req, res);
    entered.resolve();
    release.promise.then(() => res.end('finished'));
  });

  const inFlight = request(running.port, '/slow');
  await entered.promise;

  let stopped = false;
  const stopping = running.shutdown().then(() => {
    stopped = true;
  });

  await assert.rejects(request(running.port, '/'), { code: 'ECONNREFUSED' });
  assert.equal(stopped, false, 'shutdown must wait for the in-flight request');

  release.resolve();
  assert.deepEqual(await inFlight, { status: 200, body: 'finished' });
  await stopping;
  assert.equal(stopped, true);
});

test('idle keep-alive connections do not block shutdown', { timeout: 3_000 }, async () => {
  const running = await startServer(quick);
  const agent = new Agent({ keepAlive: true });
  try {
    assert.equal((await request(running.port, '/', agent)).status, 200);
    await running.shutdown();
  } finally {
    agent.destroy();
  }
});

test('a keep-alive connection that was busy during shutdown is closed afterwards', {
  timeout: 3_000,
}, async () => {
  const entered = deferred();
  const release = deferred();
  const running = await startServer((_req, res) => {
    entered.resolve();
    release.promise.then(() => res.end('late'));
  });
  const agent = new Agent({ keepAlive: true });
  try {
    const inFlight = request(running.port, '/', agent);
    await entered.promise;
    const stopping = running.shutdown();
    release.resolve();
    assert.equal((await inFlight).body, 'late');
    await stopping;
  } finally {
    agent.destroy();
  }
});

test('connections still open after the grace period are destroyed', {
  timeout: 3_000,
}, async () => {
  const entered = deferred();
  const running = await startServer(() => {
    entered.resolve(); // never answers
  });
  const stuck = request(running.port, '/hang');
  const stuckResult = assert.rejects(stuck);
  await entered.promise;

  await running.shutdown(20);
  await stuckResult;
});

test('shutdown can be called twice', async () => {
  const running = await startServer(quick);
  const first = running.shutdown();
  const second = running.shutdown();
  assert.equal(first, second);
  await first;
});
