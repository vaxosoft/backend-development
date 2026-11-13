import assert from 'node:assert/strict';
import { describe, test } from 'node:test';
import { SignJWT } from 'jose';
import { createTokenService } from './token-service.ts';

const SECRET = 'exercise-secret-exercise-secret-123';
const start = new Date('2030-01-01T00:00:00Z');

function setup(overrides: { secret?: string; issuer?: string } = {}) {
  const clock = { now: start };
  const service = createTokenService({
    secret: overrides.secret ?? SECRET,
    ttlSeconds: 60,
    now: () => clock.now,
    ...(overrides.issuer ? { issuer: overrides.issuer } : {}),
  });
  return { service, clock };
}

const key = (secret: string) => new TextEncoder().encode(secret);
const b64 = (value: object) => Buffer.from(JSON.stringify(value)).toString('base64url');

describe('createTokenService', () => {
  test('rejects secrets shorter than 32 bytes up front', () => {
    assert.throws(() => createTokenService({ secret: 'too-short' }), RangeError);
  });
});

describe('issue and verify', () => {
  test('round-trips the claims', async () => {
    const { service } = setup();
    const token = await service.issue({ sub: 7, role: 'admin' });

    assert.equal(token.split('.').length, 3);
    assert.deepEqual(await service.verify(token), { ok: true, claims: { sub: 7, role: 'admin' } });
  });

  test('is valid until exp and expired afterwards', async () => {
    const { service, clock } = setup();
    const token = await service.issue({ sub: 1, role: 'member' });

    clock.now = new Date(start.getTime() + 59_000);
    assert.equal((await service.verify(token)).ok, true);

    clock.now = new Date(start.getTime() + 61_000);
    assert.deepEqual(await service.verify(token), { ok: false, reason: 'expired' });
  });

  test('puts the standard claims in the payload', async () => {
    const { service } = setup();
    const token = await service.issue({ sub: 7, role: 'member' });
    const payload = JSON.parse(Buffer.from(token.split('.')[1] ?? '', 'base64url').toString());

    assert.equal(payload.sub, '7');
    assert.equal(payload.iss, 'task-tracker');
    assert.equal(payload.iat, start.getTime() / 1000);
    assert.equal(payload.exp, start.getTime() / 1000 + 60);
  });
});

describe('verify rejects everything else as invalid', () => {
  const invalid = { ok: false, reason: 'invalid' } as const;

  test('garbage and empty input, without throwing', async () => {
    const { service } = setup();
    for (const junk of ['', 'abc', 'a.b.c', '....']) {
      assert.deepEqual(await service.verify(junk), invalid, junk);
    }
  });

  test('tampered payload and wrong secret', async () => {
    const { service } = setup();
    const token = await service.issue({ sub: 7, role: 'member' });
    const [header, payload, signature] = token.split('.') as [string, string, string];
    const forged = { ...JSON.parse(Buffer.from(payload, 'base64url').toString()), role: 'admin' };

    assert.deepEqual(await service.verify(`${header}.${b64(forged)}.${signature}`), invalid);

    const other = setup({ secret: 'another-secret-another-secret-12345' }).service;
    assert.deepEqual(await other.verify(token), invalid);
  });

  test('alg none', async () => {
    const { service } = setup();
    const claims = {
      sub: '1',
      role: 'admin',
      iss: 'task-tracker',
      exp: start.getTime() / 1000 + 60,
    };
    assert.deepEqual(await service.verify(`${b64({ alg: 'none' })}.${b64(claims)}.`), invalid);
  });

  test('another HMAC algorithm even with the right secret', async () => {
    const { service } = setup();
    const token = await new SignJWT({ role: 'admin' })
      .setProtectedHeader({ alg: 'HS512' })
      .setSubject('1')
      .setIssuer('task-tracker')
      .setExpirationTime(Math.floor(start.getTime() / 1000) + 60)
      .sign(key(SECRET));

    assert.deepEqual(await service.verify(token), invalid);
  });

  test('wrong issuer', async () => {
    const mine = setup().service;
    const theirs = setup({ issuer: 'someone-else' }).service;

    assert.deepEqual(await mine.verify(await theirs.issue({ sub: 1, role: 'member' })), invalid);
  });

  test('validly signed tokens with bad claims', async () => {
    const { service } = setup();
    const sign = (claims: Record<string, unknown>, subject: string | undefined) => {
      const jwt = new SignJWT(claims)
        .setProtectedHeader({ alg: 'HS256' })
        .setIssuer('task-tracker')
        .setExpirationTime(Math.floor(start.getTime() / 1000) + 60);
      return (subject === undefined ? jwt : jwt.setSubject(subject)).sign(key(SECRET));
    };

    assert.deepEqual(await service.verify(await sign({ role: 'root' }, '1')), invalid);
    assert.deepEqual(await service.verify(await sign({}, '1')), invalid);
    assert.deepEqual(await service.verify(await sign({ role: 'admin' }, 'not-a-number')), invalid);
    assert.deepEqual(await service.verify(await sign({ role: 'admin' }, undefined)), invalid);
  });

  test('tokens without exp', async () => {
    const { service } = setup();
    const token = await new SignJWT({ role: 'admin' })
      .setProtectedHeader({ alg: 'HS256' })
      .setSubject('1')
      .setIssuer('task-tracker')
      .sign(key(SECRET));

    assert.deepEqual(await service.verify(token), invalid);
  });
});
