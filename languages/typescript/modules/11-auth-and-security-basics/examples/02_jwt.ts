import { errors, jwtVerify, SignJWT } from 'jose';

const secret = new TextEncoder().encode('a-demo-secret-that-is-at-least-32-bytes-long');
const issuedAt = new Date('2030-01-01T00:00:00Z');
const soon = new Date('2030-01-01T00:30:00Z');
const later = new Date('2030-01-01T02:00:00Z');

const token = await new SignJWT({ role: 'member' })
  .setProtectedHeader({ alg: 'HS256' })
  .setSubject('42')
  .setIssuer('task-tracker')
  .setIssuedAt(issuedAt)
  .setExpirationTime(new Date('2030-01-01T01:00:00Z'))
  .sign(secret);

/** Pins the algorithm and issuer; `exp` is always checked by jwtVerify. */
async function check(label: string, jwt: string, now: Date, key = secret): Promise<void> {
  try {
    const { payload, protectedHeader } = await jwtVerify(jwt, key, {
      algorithms: ['HS256'],
      issuer: 'task-tracker',
      currentDate: now,
    });
    console.log(`${label}: ok sub=${payload.sub} role=${payload.role} alg=${protectedHeader.alg}`);
  } catch (err) {
    const code = err instanceof errors.JOSEError ? err.code : 'unexpected';
    console.log(`${label}: rejected ${code}`);
  }
}

const base64url = (value: object) => Buffer.from(JSON.stringify(value)).toString('base64url');
const [header, payload, signature] = token.split('.') as [string, string, string];

const forgedPayload = base64url({
  ...JSON.parse(Buffer.from(payload, 'base64url').toString()),
  role: 'admin',
});
const unsigned = `${base64url({ alg: 'none', typ: 'JWT' })}.${payload}.`;

console.log('parts:', token.split('.').length);
await check('valid', token, soon);
await check('expired', token, later);
await check('tampered payload', `${header}.${forgedPayload}.${signature}`, soon);
await check(
  'wrong secret',
  token,
  soon,
  new TextEncoder().encode('another-secret-another-secret-123456'),
);
await check('alg none', unsigned, soon);
