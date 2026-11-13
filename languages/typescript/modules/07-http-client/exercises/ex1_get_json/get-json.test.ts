import assert from 'node:assert/strict';
import { createServer, type IncomingHttpHeaders } from 'node:http';
import type { AddressInfo } from 'node:net';
import { after, before, describe, test } from 'node:test';
import { buildUrl, getJson, HttpError } from './get-json.ts';

describe('buildUrl', () => {
  test('keeps the base path and uses exactly one slash', () => {
    assert.equal(buildUrl('http://h.test/api/', '/tasks'), 'http://h.test/api/tasks');
    assert.equal(buildUrl('http://h.test/api', 'tasks'), 'http://h.test/api/tasks');
  });

  test('appends encoded query parameters and skips undefined', () => {
    assert.equal(
      buildUrl('http://h.test', '/tasks', { status: 'todo', tag: undefined, q: 'a b&c', limit: 5 }),
      'http://h.test/tasks?status=todo&q=a+b%26c&limit=5',
    );
  });
});

describe('getJson', () => {
  let baseUrl = '';
  let lastHeaders: IncomingHttpHeaders = {};
  const server = createServer((req, res) => {
    lastHeaders = req.headers;
    if (req.url === '/ok') {
      res.setHeader('Content-Type', 'application/json');
      res.end('{"id":1,"title":"Write docs"}');
    } else if (req.url === '/boom') {
      res.writeHead(500, { 'Content-Type': 'application/json' });
      res.end('{"error":{"code":"internal"}}');
    } else {
      res.writeHead(404).end('nope');
    }
  });

  before(async () => {
    await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
    baseUrl = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
  });
  after(() => {
    server.closeAllConnections();
    server.close();
  });

  test('returns the parsed body and sends Accept', async () => {
    assert.deepEqual(await getJson(`${baseUrl}/ok`), { id: 1, title: 'Write docs' });
    assert.equal(lastHeaders.accept, 'application/json');
    assert.equal(lastHeaders.authorization, undefined);
  });

  test('sends a bearer token when given', async () => {
    await getJson(`${baseUrl}/ok`, { token: 's3cret' });
    assert.equal(lastHeaders.authorization, 'Bearer s3cret');
  });

  test('throws HttpError with status and body on non-2xx', async () => {
    await assert.rejects(getJson(`${baseUrl}/missing`), (err: unknown) => {
      assert.ok(err instanceof HttpError);
      assert.equal(err.status, 404);
      assert.equal(err.body, 'nope');
      return true;
    });
    await assert.rejects(getJson(`${baseUrl}/boom`), (err: unknown) => {
      assert.ok(err instanceof HttpError);
      assert.equal(err.status, 500);
      assert.match(err.body, /internal/);
      return true;
    });
  });
});
