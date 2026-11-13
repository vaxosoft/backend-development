import assert from 'node:assert/strict';
import { afterEach, beforeEach, describe, test } from 'node:test';
import Database from 'better-sqlite3';
import { decodeJwt, decodeProtectedHeader, jwtVerify } from 'jose';
import { login, register, SECRET, startApp, type TestApp } from './helpers.ts';

describe('auth', () => {
  let t: TestApp;
  beforeEach(async () => {
    t = await startApp();
  });
  afterEach(() => t.close());

  test('register returns 201 with id and email and never echoes the password', async () => {
    const response = await register(t.app, 'alice@example.com');

    assert.equal(response.statusCode, 201);
    const body = response.json();
    assert.equal(body.email, 'alice@example.com');
    assert.equal(typeof body.id, 'number');
    assert.ok(!response.body.includes('correct horse'));
    assert.ok(!('password' in body) && !('password_hash' in body));
  });

  test('register rejects a short password and a bad email with 422 and field messages', async () => {
    const shortPassword = await register(t.app, 'alice@example.com', 'short');
    assert.equal(shortPassword.statusCode, 422);
    assert.equal(shortPassword.json().error.code, 'validation_failed');
    assert.equal(typeof shortPassword.json().error.fields.password, 'string');

    const badEmail = await register(t.app, 'not-an-email');
    assert.equal(badEmail.statusCode, 422);
    assert.equal(typeof badEmail.json().error.fields.email, 'string');
  });

  test('registering the same email twice returns 409 conflict', async () => {
    await register(t.app, 'alice@example.com');
    const again = await register(t.app, 'ALICE@example.com');

    assert.equal(again.statusCode, 409);
    assert.equal(again.json().error.code, 'conflict');
  });

  test('malformed JSON returns 400 bad_request in the error envelope', async () => {
    const response = await t.app.inject({
      method: 'POST',
      url: '/auth/register',
      headers: { 'content-type': 'application/json' },
      payload: '{"email": ',
    });

    assert.equal(response.statusCode, 400);
    assert.match(String(response.headers['content-type']), /^application\/json/);
    assert.equal(response.json().error.code, 'bad_request');
  });

  test('passwords are stored as salted hashes, never in clear text', async () => {
    await register(t.app, 'alice@example.com', 'same password');
    await register(t.app, 'bob@example.com', 'same password');

    const db = new Database(t.dbPath, { readonly: true });
    const rows = db.prepare('SELECT password_hash FROM users ORDER BY id').all() as {
      password_hash: string;
    }[];
    db.close();

    assert.equal(rows.length, 2);
    const [first, second] = rows.map((row) => row.password_hash);
    assert.ok(first && second);
    assert.ok(!first.includes('same password'));
    assert.notEqual(first, second, 'a random salt per password gives different hashes');
  });

  test('login returns an HS256 JWT that expires in one hour', async () => {
    await register(t.app, 'alice@example.com');
    const response = await login(t.app, 'alice@example.com');

    assert.equal(response.statusCode, 200);
    const { token } = response.json<{ token: string }>();
    assert.equal(decodeProtectedHeader(token).alg, 'HS256');
    const claims = decodeJwt(token);
    assert.equal(Number(claims.exp) - Number(claims.iat), 3600);
    await jwtVerify(token, new TextEncoder().encode(SECRET), { algorithms: ['HS256'] });
  });

  test('wrong password and unknown user get the identical 401', async () => {
    await register(t.app, 'alice@example.com');
    const wrongPassword = await login(t.app, 'alice@example.com', 'not the password');
    const unknownUser = await login(t.app, 'nobody@example.com');

    assert.equal(wrongPassword.statusCode, 401);
    assert.equal(unknownUser.statusCode, 401);
    assert.equal(wrongPassword.json().error.code, 'unauthorized');
    assert.deepEqual(unknownUser.json(), wrongPassword.json());
  });
});
