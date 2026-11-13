import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import { hashPassword, needsRehash, verifyPassword } from './password.ts';

// A low cost keeps the tests fast; production uses the default.
const COST = 1024;

describe('hashPassword', () => {
  test('uses the documented format and never contains the password', async () => {
    const stored = await hashPassword('s3cret-pass', COST);
    const parts = stored.split('$');

    assert.equal(parts.length, 4);
    assert.equal(parts[0], 'scrypt');
    assert.equal(parts[1], '1024');
    assert.equal(Buffer.from(parts[2] ?? '', 'base64url').length, 16);
    assert.equal(Buffer.from(parts[3] ?? '', 'base64url').length, 32);
    assert.ok(!stored.includes('s3cret-pass'));
  });

  test('uses a fresh salt every time', async () => {
    const [a, b] = await Promise.all([hashPassword('same', COST), hashPassword('same', COST)]);
    assert.notEqual(a, b);
  });

  test('defaults to cost 16384', async () => {
    assert.equal((await hashPassword('x')).split('$')[1], '16384');
  });
});

describe('verifyPassword', () => {
  test('accepts the right password and rejects others', async () => {
    const stored = await hashPassword('correct horse', COST);

    assert.equal(await verifyPassword('correct horse', stored), true);
    assert.equal(await verifyPassword('Correct horse', stored), false);
    assert.equal(await verifyPassword('', stored), false);
  });

  test('reads the cost from the stored hash', async () => {
    const low = await hashPassword('pw', 1024);
    const high = await hashPassword('pw', 2048);

    assert.equal(await verifyPassword('pw', low), true);
    assert.equal(await verifyPassword('pw', high), true);
  });

  test('treats canonically equivalent Unicode as the same password', async () => {
    const stored = await hashPassword('caf\u00e9', COST);
    assert.equal(await verifyPassword('cafe\u0301', stored), true);
  });

  test('returns false for malformed stored hashes instead of throwing', async () => {
    const good = await hashPassword('pw', COST);
    const [scheme, cost, salt, key] = good.split('$') as [string, string, string, string];
    const malformed = [
      '',
      'not-a-hash',
      `bcrypt$${cost}$${salt}$${key}`,
      `${scheme}$abc$${salt}$${key}`,
      `${scheme}$${cost}$${salt}`,
      `${scheme}$${cost}$${salt}$${key}$extra`,
      `${scheme}$${cost}$${salt}$${key.slice(0, 10)}`,
      `${scheme}$${cost}$${salt}$`,
    ];

    for (const stored of malformed) {
      assert.equal(await verifyPassword('pw', stored), false, JSON.stringify(stored));
    }
  });
});

describe('needsRehash', () => {
  test('flags hashes made with a lower cost than the target', async () => {
    const stored = await hashPassword('pw', 1024);

    assert.equal(needsRehash(stored, 1024), false);
    assert.equal(needsRehash(stored, 512), false);
    assert.equal(needsRehash(stored, 2048), true);
  });

  test('flags malformed hashes', () => {
    assert.equal(needsRehash('nonsense', 1024), true);
    assert.equal(needsRehash('scrypt$x$a$b', 1024), true);
  });
});
