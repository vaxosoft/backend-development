import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { mkdtemp, readdir, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { afterEach, beforeEach, describe, test } from 'node:test';
import type { FastifyInstance, LightMyRequestResponse } from 'fastify';
import { buildApp } from '../starter/src/app.ts';
import { sanitizeFilename } from '../starter/src/filename.ts';

let root: string;
let storageDir: string;
const open: FastifyInstance[] = [];

beforeEach(async () => {
  root = await mkdtemp(join(tmpdir(), 'p3-files-'));
  storageDir = join(root, 'storage');
});

afterEach(async () => {
  // fetch keeps connections alive; closing them explicitly keeps app.close() from waiting.
  for (const app of open) app.server.closeAllConnections();
  await Promise.all(open.splice(0).map((app) => app.close()));
  await rm(root, { recursive: true, force: true });
});

async function newApp(maxFileSize?: number): Promise<FastifyInstance> {
  const app = await buildApp(
    maxFileSize === undefined ? { storageDir } : { storageDir, maxFileSize },
  );
  open.push(app);
  return app;
}

const sha256 = (data: Uint8Array) => createHash('sha256').update(data).digest('hex');

interface UploadOptions {
  field?: string;
  name?: string;
  type?: string;
}

async function multipartRequest(
  content: string | Uint8Array<ArrayBuffer>,
  options: UploadOptions = {},
) {
  const form = new FormData();
  const blob = new Blob([content], options.type ? { type: options.type } : {});
  form.append(options.field ?? 'file', blob, options.name ?? 'upload.bin');
  const encoded = new Response(form);
  return {
    headers: { 'content-type': String(encoded.headers.get('content-type')) },
    payload: Buffer.from(await encoded.arrayBuffer()),
  };
}

async function upload(
  app: FastifyInstance,
  content: string | Uint8Array<ArrayBuffer>,
  options: UploadOptions = {},
): Promise<LightMyRequestResponse> {
  const { headers, payload } = await multipartRequest(content, options);
  return app.inject({ method: 'POST', url: '/files', headers, payload });
}

const storedFiles = async () => readdir(storageDir);

function assertEnvelope(res: LightMyRequestResponse, status: number, code: string): void {
  assert.equal(res.statusCode, status, res.body);
  assert.match(String(res.headers['content-type']), /^application\/json/);
  assert.equal(res.json().error.code, code);
}

describe('POST /files', () => {
  test('stores the file and answers 201 with id, name, size, sha256 and content_type', async () => {
    const app = await newApp();
    const content = 'hello, files\n';
    const res = await upload(app, content, { name: 'hello.txt', type: 'text/plain' });
    assert.equal(res.statusCode, 201);
    const body = res.json();
    assert.deepEqual(Object.keys(body).sort(), ['content_type', 'id', 'name', 'sha256', 'size']);
    assert.match(body.id, /^[0-9a-f]{32}$/);
    assert.equal(body.name, 'hello.txt');
    assert.equal(body.size, content.length);
    assert.equal(body.sha256, sha256(Buffer.from(content)));
    assert.equal(body.content_type, 'text/plain');
    assert.equal(res.headers.location, `/files/${body.id}`);
  });

  test('each upload gets its own generated id, even for the same name and content', async () => {
    const app = await newApp();
    const a = (await upload(app, 'same', { name: 'same.txt' })).json();
    const b = (await upload(app, 'same', { name: 'same.txt' })).json();
    assert.notEqual(a.id, b.id);
  });

  test('files live under generated ids in the storage directory, not under their own name', async () => {
    const app = await newApp();
    const { id } = (await upload(app, 'data', { name: 'report.pdf' })).json();
    const entries = await storedFiles();
    assert.ok(entries.length > 0);
    assert.ok(
      entries.every((entry) => entry.startsWith(id)),
      entries.join(','),
    );
    assert.ok(!entries.some((entry) => entry.includes('report')));
  });

  test('a traversal name like ../../etc/passwd is sanitized and cannot escape the storage dir', async () => {
    const app = await newApp();
    const res = await upload(app, 'x', { name: '../../etc/passwd' });
    assert.equal(res.statusCode, 201);
    assert.equal(res.json().name, 'passwd');
    assert.deepEqual(await readdir(root), ['storage']);
    assert.ok((await storedFiles()).every((entry) => entry.startsWith(res.json().id)));
  });

  test('sanitizeFilename removes separators, "..", control characters and quotes', () => {
    const cases: [string | undefined, string][] = [
      ['report.pdf', 'report.pdf'],
      ['../../etc/passwd', 'passwd'],
      ['..\\..\\windows\\win.ini', 'win.ini'],
      ['a/b/c.txt', 'c.txt'],
      ['bad\u0000na\nme\t.txt', 'badname.txt'],
      ['say "hi".txt', 'say hi.txt'],
      ['archive..tar..gz', 'archive.tar.gz'],
      ['..', 'file'],
      ['', 'file'],
      ['folder/', 'file'],
      [undefined, 'file'],
      ['  spaced.txt  ', 'spaced.txt'],
      ['résumé.pdf', 'résumé.pdf'],
      ['x'.repeat(300), 'x'.repeat(255)],
    ];
    for (const [input, expected] of cases) {
      assert.equal(sanitizeFilename(input), expected, JSON.stringify(input));
    }
  });

  test('the default limit is 5 MiB: 2 MiB is accepted, one byte over 5 MiB is a 413', async () => {
    const app = await newApp();
    assert.equal((await upload(app, new Uint8Array(2 * 1024 * 1024))).statusCode, 201);
    const tooBig = await upload(app, new Uint8Array(5 * 1024 * 1024 + 1));
    assertEnvelope(tooBig, 413, 'payload_too_large');
  });

  test('a file of exactly the configured maximum is accepted', async () => {
    const app = await newApp(1024);
    const res = await upload(app, new Uint8Array(1024));
    assert.equal(res.statusCode, 201);
    assert.equal(res.json().size, 1024);
  });

  test('an oversized upload is a 413 payload_too_large and leaves nothing on disk', async () => {
    const app = await newApp(1024);
    const res = await upload(app, new Uint8Array(1025));
    assertEnvelope(res, 413, 'payload_too_large');
    assert.deepEqual(await storedFiles(), []);
  });

  test('a request without a "file" part is a 400 bad_request', async () => {
    const app = await newApp();
    assertEnvelope(await upload(app, 'x', { field: 'document' }), 400, 'bad_request');
    const textOnly = new FormData();
    textOnly.append('note', 'no file here');
    const encoded = new Response(textOnly);
    const res = await app.inject({
      method: 'POST',
      url: '/files',
      headers: { 'content-type': String(encoded.headers.get('content-type')) },
      payload: Buffer.from(await encoded.arrayBuffer()),
    });
    assertEnvelope(res, 400, 'bad_request');
  });

  test('a body that is not multipart is a 415 unsupported_media_type', async () => {
    const app = await newApp();
    const res = await app.inject({
      method: 'POST',
      url: '/files',
      headers: { 'content-type': 'text/plain' },
      payload: 'raw',
    });
    assertEnvelope(res, 415, 'unsupported_media_type');
  });
});

describe('GET /files/{id}', () => {
  test('streams the bytes back with Content-Length, Content-Type, Content-Disposition and ETag', async () => {
    const app = await newApp();
    const bytes = Uint8Array.from({ length: 256 }, (_, i) => i);
    const { id, sha256: digest } = (
      await upload(app, bytes, { name: 'all-bytes.png', type: 'image/png' })
    ).json();

    const res = await app.inject(`/files/${id}`);
    assert.equal(res.statusCode, 200);
    assert.deepEqual(res.rawPayload, Buffer.from(bytes));
    assert.equal(res.headers['content-length'], '256');
    assert.equal(res.headers['content-type'], 'image/png');
    assert.equal(res.headers['content-disposition'], 'attachment; filename="all-bytes.png"');
    assert.equal(res.headers.etag, `"${digest}"`);
  });

  test('non-ASCII names get an ASCII fallback plus filename*', async () => {
    const app = await newApp();
    const { id } = (await upload(app, 'x', { name: 'résumé.pdf' })).json();
    const res = await app.inject(`/files/${id}`);
    assert.equal(res.statusCode, 200);
    assert.equal(
      res.headers['content-disposition'],
      `attachment; filename="r_sum_.pdf"; filename*=UTF-8''r%C3%A9sum%C3%A9.pdf`,
    );
  });

  test('If-None-Match with the current ETag answers 304 with no body', async () => {
    const app = await newApp();
    const { id, sha256: digest } = (await upload(app, 'cache me')).json();
    const etag = `"${digest}"`;

    for (const header of [etag, `W/${etag}`, `"other", ${etag}`, '*']) {
      const res = await app.inject({ url: `/files/${id}`, headers: { 'if-none-match': header } });
      assert.equal(res.statusCode, 304, header);
      assert.equal(res.body, '');
      assert.equal(res.headers.etag, etag);
    }

    const stale = await app.inject({
      url: `/files/${id}`,
      headers: { 'if-none-match': '"stale"' },
    });
    assert.equal(stale.statusCode, 200);
    assert.equal(stale.body, 'cache me');
  });

  test('unknown and malformed ids are 404 not_found, never a file lookup', async () => {
    const app = await newApp();
    const ids = ['0123456789abcdef0123456789abcdef', 'nope', '..%2F..%2Fetc%2Fpasswd', 'ABC'];
    for (const id of ids) {
      assertEnvelope(await app.inject(`/files/${id}`), 404, 'not_found');
    }
  });
});

describe('DELETE /files/{id}', () => {
  test('answers 204, removes everything from disk and the file is gone', async () => {
    const app = await newApp();
    const { id } = (await upload(app, 'bye')).json();
    const res = await app.inject({ method: 'DELETE', url: `/files/${id}` });
    assert.equal(res.statusCode, 204);
    assert.equal(res.body, '');
    assertEnvelope(await app.inject(`/files/${id}`), 404, 'not_found');
    assert.deepEqual(await storedFiles(), []);
    assertEnvelope(await app.inject({ method: 'DELETE', url: `/files/${id}` }), 404, 'not_found');
  });
});

describe('over a real socket', () => {
  test('a 3 MiB upload and download keep their bytes and SHA-256', async () => {
    const app = await newApp();
    const base = await app.listen({ host: '127.0.0.1', port: 0 });
    const bytes = new Uint8Array(3 * 1024 * 1024).map((_, i) => (i * 31) % 251);

    const form = new FormData();
    form.append('file', new Blob([bytes], { type: 'application/octet-stream' }), 'big.bin');
    const created = await fetch(`${base}/files`, { method: 'POST', body: form });
    assert.equal(created.status, 201);
    const record = (await created.json()) as { id: string; size: number; sha256: string };
    assert.equal(record.size, bytes.length);
    assert.equal(record.sha256, sha256(bytes));

    const downloaded = await fetch(`${base}/files/${record.id}`);
    assert.equal(downloaded.headers.get('content-length'), String(bytes.length));
    assert.equal(sha256(new Uint8Array(await downloaded.arrayBuffer())), record.sha256);
  });
});
