import assert from 'node:assert/strict';
import { afterEach, beforeEach, describe, test } from 'node:test';
import { SignJWT } from 'jose';
import { bearer, createLink, SECRET, signUp, startApp, type TestApp } from './helpers.ts';

const key = new TextEncoder().encode(SECRET);

function unsignedToken(): string {
  const encode = (value: object) => Buffer.from(JSON.stringify(value)).toString('base64url');
  const exp = Math.floor(Date.now() / 1000) + 3600;
  return `${encode({ alg: 'none', typ: 'JWT' })}.${encode({ sub: '1', exp })}.`;
}

describe('links', () => {
  let t: TestApp;
  let alice: string;
  beforeEach(async () => {
    t = await startApp();
    alice = await signUp(t.app, 'alice@example.com');
  });
  afterEach(() => t.close());

  describe('authentication', () => {
    test('protected endpoints answer 401 without a token', async () => {
      const calls = [
        { method: 'POST', url: '/links', payload: { url: 'https://example.com' } },
        { method: 'GET', url: '/links' },
        { method: 'DELETE', url: '/links/abc' },
      ] as const;
      for (const call of calls) {
        const response = await t.app.inject(call);
        assert.equal(response.statusCode, 401, `${call.method} ${call.url}`);
        assert.equal(response.json().error.code, 'unauthorized');
      }
    });

    test('forged, expired and wrongly signed tokens are rejected', async () => {
      const now = Math.floor(Date.now() / 1000);
      const expired = await new SignJWT()
        .setProtectedHeader({ alg: 'HS256' })
        .setSubject('1')
        .setIssuedAt(now - 7200)
        .setExpirationTime(now - 3600)
        .sign(key);
      const otherKey = await new SignJWT()
        .setProtectedHeader({ alg: 'HS256' })
        .setSubject('1')
        .setExpirationTime('1h')
        .sign(new TextEncoder().encode('another-secret-that-is-also-32-chars-long'));
      const hs384 = await new SignJWT()
        .setProtectedHeader({ alg: 'HS384' })
        .setSubject('1')
        .setExpirationTime('1h')
        .sign(key);

      for (const token of ['garbage', expired, otherKey, hs384, unsignedToken()]) {
        const response = await t.app.inject({
          method: 'GET',
          url: '/links',
          headers: bearer(token),
        });
        assert.equal(response.statusCode, 401, token.slice(0, 20));
      }
    });
  });

  describe('creating links', () => {
    test('generates a 7 character alphanumeric slug', async () => {
      const response = await createLink(t.app, alice, { url: 'https://example.com/docs' });

      assert.equal(response.statusCode, 201);
      const body = response.json();
      assert.match(body.slug, /^[A-Za-z0-9]{7}$/);
      assert.equal(body.url, 'https://example.com/docs');
    });

    test('uses a custom slug when given', async () => {
      const response = await createLink(t.app, alice, {
        url: 'http://example.com',
        slug: 'my_link-1',
      });

      assert.equal(response.statusCode, 201);
      assert.deepEqual(response.json(), { slug: 'my_link-1', url: 'http://example.com' });
    });

    test('rejects URLs that are not absolute http(s) with 422 on the url field', async () => {
      for (const url of [
        'example.com',
        '/relative',
        'ftp://example.com/file',
        'javascript:alert(1)',
      ]) {
        const response = await createLink(t.app, alice, { url });
        assert.equal(response.statusCode, 422, url);
        assert.equal(response.json().error.code, 'validation_failed');
        assert.equal(typeof response.json().error.fields.url, 'string');
      }
      assert.equal((await createLink(t.app, alice, {})).statusCode, 422);
    });

    test('rejects slugs outside 3-32 characters of A-Z a-z 0-9 _ - with 422', async () => {
      for (const slug of ['ab', 'a'.repeat(33), 'has space', 'dots.not.ok', 'sláug']) {
        const response = await createLink(t.app, alice, { url: 'https://example.com', slug });
        assert.equal(response.statusCode, 422, slug);
        assert.equal(typeof response.json().error.fields.slug, 'string');
      }
    });

    test('a taken slug returns 409 conflict, also for another user', async () => {
      const bob = await signUp(t.app, 'bob@example.com');
      await createLink(t.app, alice, { url: 'https://example.com/a', slug: 'taken' });

      const sameUser = await createLink(t.app, alice, {
        url: 'https://example.com/b',
        slug: 'taken',
      });
      const otherUser = await createLink(t.app, bob, {
        url: 'https://example.com/c',
        slug: 'taken',
      });

      assert.equal(sameUser.statusCode, 409);
      assert.equal(otherUser.statusCode, 409);
      assert.equal(otherUser.json().error.code, 'conflict');
    });

    test('the slug "links" is reserved because GET /links is the listing', async () => {
      const response = await createLink(t.app, alice, {
        url: 'https://example.com',
        slug: 'links',
      });

      assert.equal(response.statusCode, 409);
    });
  });

  describe('redirecting', () => {
    test('GET /{slug} is public and answers 302 with Location', async () => {
      await createLink(t.app, alice, { url: 'https://example.com/docs?x=1', slug: 'docs' });
      const response = await t.app.inject({ method: 'GET', url: '/docs' });

      assert.equal(response.statusCode, 302);
      assert.equal(response.headers.location, 'https://example.com/docs?x=1');
    });

    test('an unknown slug returns 404 in the error envelope', async () => {
      const response = await t.app.inject({ method: 'GET', url: '/nope' });

      assert.equal(response.statusCode, 404);
      assert.equal(response.json().error.code, 'not_found');
    });
  });

  describe('listing and deleting', () => {
    test('GET /links lists only my links, with hits counted per visit', async () => {
      const bob = await signUp(t.app, 'bob@example.com');
      await createLink(t.app, alice, { url: 'https://example.com/1', slug: 'one' });
      await createLink(t.app, alice, { url: 'https://example.com/2', slug: 'two' });
      await createLink(t.app, bob, { url: 'https://example.com/3', slug: 'three' });
      await t.app.inject({ method: 'GET', url: '/one' });
      await t.app.inject({ method: 'GET', url: '/one' });
      await t.app.inject({ method: 'GET', url: '/three' });

      const response = await t.app.inject({ method: 'GET', url: '/links', headers: bearer(alice) });

      assert.equal(response.statusCode, 200);
      const body = response.json();
      assert.deepEqual(
        body.items.map((l: { slug: string; hits: number }) => [l.slug, l.hits]),
        [
          ['one', 2],
          ['two', 0],
        ],
      );
      assert.equal(body.items[0].url, 'https://example.com/1');
      assert.equal(body.total, 2);
    });

    test('GET /links pages with limit and offset and clamps limit to 100', async () => {
      for (const slug of ['aaa', 'bbb', 'ccc']) {
        await createLink(t.app, alice, { url: 'https://example.com', slug });
      }
      const page = await t.app.inject({
        method: 'GET',
        url: '/links?limit=1&offset=1',
        headers: bearer(alice),
      });
      const clamped = await t.app.inject({
        method: 'GET',
        url: '/links?limit=1000',
        headers: bearer(alice),
      });
      const invalid = await t.app.inject({
        method: 'GET',
        url: '/links?limit=abc',
        headers: bearer(alice),
      });

      assert.deepEqual(
        page.json().items.map((l: { slug: string }) => l.slug),
        ['bbb'],
      );
      assert.equal(page.json().total, 3);
      assert.equal(page.json().limit, 1);
      assert.equal(page.json().offset, 1);
      assert.equal(clamped.json().limit, 100);
      assert.equal(invalid.statusCode, 400);
    });

    test('the owner can delete a link and it stops redirecting', async () => {
      await createLink(t.app, alice, { url: 'https://example.com', slug: 'gone' });
      const response = await t.app.inject({
        method: 'DELETE',
        url: '/links/gone',
        headers: bearer(alice),
      });

      assert.equal(response.statusCode, 204);
      assert.equal(response.body, '');
      assert.equal((await t.app.inject({ method: 'GET', url: '/gone' })).statusCode, 404);
    });

    test('another user gets 403 and the link survives', async () => {
      const bob = await signUp(t.app, 'bob@example.com');
      await createLink(t.app, alice, { url: 'https://example.com', slug: 'mine' });

      const response = await t.app.inject({
        method: 'DELETE',
        url: '/links/mine',
        headers: bearer(bob),
      });

      assert.equal(response.statusCode, 403);
      assert.equal(response.json().error.code, 'forbidden');
      assert.equal((await t.app.inject({ method: 'GET', url: '/mine' })).statusCode, 302);
    });

    test('deleting an unknown slug returns 404', async () => {
      const response = await t.app.inject({
        method: 'DELETE',
        url: '/links/nope',
        headers: bearer(alice),
      });

      assert.equal(response.statusCode, 404);
    });
  });
});
