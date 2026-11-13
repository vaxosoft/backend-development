import assert from 'node:assert/strict';
import { performance } from 'node:perf_hooks';
import { type TestContext, test } from 'node:test';
import { setTimeout as sleep } from 'node:timers/promises';
import type { FastifyInstance } from 'fastify';
import pino from 'pino';
import { buildApp } from '../starter/src/app.ts';
import type { Config } from '../starter/src/config.ts';
import type { Job } from '../starter/src/types.ts';

async function newApp(t: TestContext, overrides: Partial<Config> = {}): Promise<FastifyInstance> {
  const app = await buildApp({ retryDelayMs: 1, drainTimeoutMs: 20, ...overrides });
  t.after(() => app.close());
  return app;
}

async function submit(app: FastifyInstance, body: unknown): Promise<number> {
  const res = await app.inject({ method: 'POST', url: '/jobs', payload: body as object });
  assert.equal(res.statusCode, 202, res.body);
  return res.json<{ id: number }>().id;
}

async function getJob(app: FastifyInstance, id: number): Promise<Job> {
  const res = await app.inject({ url: `/jobs/${id}` });
  assert.equal(res.statusCode, 200, res.body);
  return res.json<Job>();
}

async function waitFor(app: FastifyInstance, id: number, status: Job['status']): Promise<Job> {
  for (let i = 0; i < 400; i++) {
    const job = await getJob(app, id);
    if (job.status === status) {
      return job;
    }
    await sleep(5);
  }
  return assert.fail(`job ${id} never became ${status}`);
}

const longSleep = { type: 'sleep', payload: { ms: 2000 } };

test('POST /jobs accepts a job with 202 and a numeric id', async (t) => {
  const app = await newApp(t);
  const res = await app.inject({
    method: 'POST',
    url: '/jobs',
    payload: { type: 'reverse', payload: { text: 'abc' } },
  });
  assert.equal(res.statusCode, 202);
  assert.equal(typeof res.json<{ id: unknown }>().id, 'number');
});

test('a reverse job succeeds with the reversed text', async (t) => {
  const app = await newApp(t);
  const id = await submit(app, { type: 'reverse', payload: { text: 'hello' } });
  assert.deepEqual(await waitFor(app, id, 'succeeded'), {
    id,
    type: 'reverse',
    status: 'succeeded',
    attempts: 1,
    result: 'olleh',
    error: null,
  });
});

test('reverse works on code points, not UTF-16 units', async (t) => {
  const app = await newApp(t);
  const id = await submit(app, { type: 'reverse', payload: { text: 'a😀b' } });
  assert.equal((await waitFor(app, id, 'succeeded')).result, 'b😀a');
});

test('a sleep job succeeds after waiting', async (t) => {
  const app = await newApp(t);
  const id = await submit(app, { type: 'sleep', payload: { ms: 10 } });
  const job = await waitFor(app, id, 'succeeded');
  assert.equal(job.result, 'slept 10ms');
});

test('a failing job is retried up to max_attempts and then fails', async (t) => {
  const app = await newApp(t, { maxAttempts: 3 });
  const id = await submit(app, { type: 'fail', payload: {} });
  const job = await waitFor(app, id, 'failed');
  assert.equal(job.attempts, 3);
  assert.equal(job.error, 'job failed on purpose');
  assert.equal(job.result, null);
});

test('retries wait for the configured delay', async (t) => {
  const app = await newApp(t, { maxAttempts: 2, retryDelayMs: 60 });
  const started = performance.now();
  const id = await submit(app, { type: 'fail' });
  await waitFor(app, id, 'failed');
  assert.ok(performance.now() - started >= 50);
});

test('a job that exceeds the job timeout fails with a timeout error', async (t) => {
  const app = await newApp(t, { jobTimeoutMs: 20, maxAttempts: 1 });
  const id = await submit(app, { type: 'sleep', payload: { ms: 2000 } });
  const job = await waitFor(app, id, 'failed');
  assert.match(job.error ?? '', /timed out/);
});

test('GET /jobs/{id} answers 404 with the error envelope for an unknown job', async (t) => {
  const app = await newApp(t);
  const res = await app.inject({ url: '/jobs/999' });
  assert.equal(res.statusCode, 404);
  assert.equal(res.json<{ error: { code: string } }>().error.code, 'not_found');
});

test('GET /jobs lists the newest job first', async (t) => {
  const app = await newApp(t);
  const first = await submit(app, { type: 'reverse', payload: { text: 'a' } });
  const second = await submit(app, { type: 'reverse', payload: { text: 'b' } });
  const res = await app.inject({ url: '/jobs' });
  const body = res.json<{ items: Job[]; total: number }>();
  assert.deepEqual(
    body.items.map((job) => job.id),
    [second, first],
  );
  assert.equal(body.total, 2);
});

test('an unknown job type is a 422 validation error naming the field', async (t) => {
  const app = await newApp(t);
  const res = await app.inject({ method: 'POST', url: '/jobs', payload: { type: 'explode' } });
  assert.equal(res.statusCode, 422);
  const { error } = res.json<{ error: { code: string; fields: Record<string, string> } }>();
  assert.equal(error.code, 'validation_failed');
  assert.ok('type' in error.fields);
});

test('sleep longer than 5000 ms is rejected with 422', async (t) => {
  const app = await newApp(t);
  const res = await app.inject({
    method: 'POST',
    url: '/jobs',
    payload: { type: 'sleep', payload: { ms: 5001 } },
  });
  assert.equal(res.statusCode, 422);
  assert.ok('payload.ms' in res.json<{ error: { fields: object } }>().error.fields);
});

test('malformed JSON is a 400 bad_request', async (t) => {
  const app = await newApp(t);
  const res = await app.inject({
    method: 'POST',
    url: '/jobs',
    headers: { 'content-type': 'application/json' },
    payload: '{"type": ',
  });
  assert.equal(res.statusCode, 400);
  assert.equal(res.json<{ error: { code: string } }>().error.code, 'bad_request');
});

test('a full queue answers 429 rate_limited with Retry-After', async (t) => {
  const app = await newApp(t, { workers: 1, queueCapacity: 1 });
  await submit(app, longSleep);
  await submit(app, longSleep);
  const res = await app.inject({ method: 'POST', url: '/jobs', payload: longSleep });
  assert.equal(res.statusCode, 429);
  assert.equal(res.json<{ error: { code: string } }>().error.code, 'rate_limited');
  assert.ok(res.headers['retry-after']);
});

test('at most `workers` jobs run at the same time', async (t) => {
  const app = await newApp(t, { workers: 2 });
  const ids = [
    await submit(app, longSleep),
    await submit(app, longSleep),
    await submit(app, longSleep),
  ];
  await waitFor(app, ids[0] ?? 0, 'running');
  await waitFor(app, ids[1] ?? 0, 'running');
  assert.equal((await getJob(app, ids[2] ?? 0)).status, 'queued');
});

test('DELETE cancels a queued job so it never runs', async (t) => {
  const app = await newApp(t, { workers: 1 });
  await submit(app, longSleep);
  const queued = await submit(app, { type: 'reverse', payload: { text: 'x' } });
  const res = await app.inject({ method: 'DELETE', url: `/jobs/${queued}` });
  assert.equal(res.statusCode, 204);
  const job = await getJob(app, queued);
  assert.equal(job.status, 'cancelled');
  assert.equal(job.attempts, 0);
});

test('DELETE cancels a running job and frees its worker', async (t) => {
  const app = await newApp(t, { workers: 1 });
  const running = await submit(app, longSleep);
  const next = await submit(app, { type: 'reverse', payload: { text: 'ok' } });
  await waitFor(app, running, 'running');
  const res = await app.inject({ method: 'DELETE', url: `/jobs/${running}` });
  assert.equal(res.statusCode, 204);
  assert.equal((await getJob(app, running)).status, 'cancelled');
  assert.equal((await waitFor(app, next, 'succeeded')).result, 'ko');
});

test('DELETE on a finished job is 409 conflict', async (t) => {
  const app = await newApp(t);
  const id = await submit(app, { type: 'reverse', payload: { text: 'a' } });
  await waitFor(app, id, 'succeeded');
  const res = await app.inject({ method: 'DELETE', url: `/jobs/${id}` });
  assert.equal(res.statusCode, 409);
  assert.equal(res.json<{ error: { code: string } }>().error.code, 'conflict');
});

test('DELETE on an unknown job is 404', async (t) => {
  const app = await newApp(t);
  const res = await app.inject({ method: 'DELETE', url: '/jobs/42' });
  assert.equal(res.statusCode, 404);
});

test('closing the app lets running and queued jobs finish first', async (t) => {
  const app = await newApp(t, { workers: 1, drainTimeoutMs: 5000 });
  const first = await submit(app, { type: 'sleep', payload: { ms: 30 } });
  const second = await submit(app, { type: 'sleep', payload: { ms: 30 } });
  await app.close();
  assert.equal(app.pool.get(first)?.status, 'succeeded');
  assert.equal(app.pool.get(second)?.status, 'succeeded');
});

test('jobs still unfinished after the drain timeout are cancelled', async (t) => {
  const app = await newApp(t, { workers: 1, drainTimeoutMs: 20 });
  const running = await submit(app, longSleep);
  const queued = await submit(app, longSleep);
  const started = performance.now();
  await app.close();
  assert.ok(performance.now() - started < 1500);
  assert.equal(app.pool.get(running)?.status, 'cancelled');
  assert.equal(app.pool.get(queued)?.status, 'cancelled');
});

test('a pool that is shutting down rejects new jobs with 503', async (t) => {
  const app = await newApp(t);
  await app.pool.shutdown();
  const res = await app.inject({ method: 'POST', url: '/jobs', payload: longSleep });
  assert.equal(res.statusCode, 503);
});

test('over a real socket, close() finishes the running job and then refuses connections', async (t) => {
  const app = await newApp(t, { drainTimeoutMs: 5000 });
  const base = await app.listen({ host: '127.0.0.1', port: 0 });
  const res = await fetch(`${base}/jobs`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ type: 'sleep', payload: { ms: 40 } }),
  });
  const { id } = (await res.json()) as { id: number };
  await app.close();
  assert.equal(app.pool.get(id)?.status, 'succeeded');
  await assert.rejects(fetch(`${base}/jobs`));
});

test('each processed job logs one structured "job finished" line', async (t) => {
  const lines: Array<Record<string, unknown>> = [];
  const logger = pino({ level: 'info' }, { write: (line: string) => lines.push(JSON.parse(line)) });
  const app = await newApp(t, { logger });
  const id = await submit(app, { type: 'reverse', payload: { text: 'log' } });
  await waitFor(app, id, 'succeeded');
  const line = lines.find((entry) => entry.msg === 'job finished');
  assert.ok(line, 'no "job finished" line was logged');
  assert.equal(line.jobId, id);
  assert.equal(line.status, 'succeeded');
  assert.equal(line.attempts, 1);
  assert.equal(typeof line.durationMs, 'number');
});
