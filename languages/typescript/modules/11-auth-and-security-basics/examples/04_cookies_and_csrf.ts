import { createHmac, timingSafeEqual } from 'node:crypto';
import fastifyCookie from '@fastify/cookie';
import Fastify from 'fastify';

const SECRET = 'demo-cookie-secret-demo-cookie-secret';

const app = Fastify();
await app.register(fastifyCookie, { secret: SECRET });

// The CSRF token is derived from the session id, so the server stores nothing extra.
const csrfTokenFor = (sessionId: string) =>
  createHmac('sha256', SECRET).update(`csrf:${sessionId}`).digest('base64url');

function sameToken(a: string, b: string): boolean {
  const left = Buffer.from(a);
  const right = Buffer.from(b);
  return left.length === right.length && timingSafeEqual(left, right);
}

app.post('/login', async (_req, reply) => {
  const sessionId = 'session-1'; // a real server uses randomBytes(32) and stores the session
  reply.setCookie('sid', sessionId, {
    httpOnly: true, // not readable from JavaScript: an XSS bug cannot steal it
    secure: true, // only sent over HTTPS
    sameSite: 'lax', // not sent on cross-site POSTs
    path: '/',
    maxAge: 3600,
    signed: true, // tamper-evident
  });
  return { csrfToken: csrfTokenFor(sessionId) };
});

app.addHook('preHandler', async (req, reply) => {
  if (req.method === 'GET' || req.url === '/login') return;
  const raw = req.cookies.sid;
  const session = raw ? req.unsignCookie(raw) : undefined;
  if (!session?.valid)
    return reply.code(401).send({ error: { code: 'unauthorized', message: 'login required' } });
  const sent = req.headers['x-csrf-token'];
  if (typeof sent !== 'string' || !sameToken(sent, csrfTokenFor(session.value))) {
    return reply
      .code(403)
      .send({ error: { code: 'forbidden', message: 'csrf token missing or wrong' } });
  }
});

app.post('/tasks', async (_req, reply) => reply.code(201).send({ created: true }));

const login = await app.inject({ method: 'POST', url: '/login' });
const setCookie = String(login.headers['set-cookie']);
const [pair = '', ...attributes] = setCookie.split('; ');
console.log('cookie attributes:', attributes.join(' | '));
const cookie = pair;
const { csrfToken } = login.json<{ csrfToken: string }>();

const attempt = async (label: string, headers: Record<string, string>) => {
  const res = await app.inject({ method: 'POST', url: '/tasks', headers });
  console.log(`${label}: ${res.statusCode} ${res.body}`);
};

await attempt('no cookie', {});
await attempt('cookie only (forged cross-site form)', { cookie });
await attempt('cookie + wrong token', { cookie, 'x-csrf-token': 'guess' });
await attempt('cookie + token', { cookie, 'x-csrf-token': csrfToken });
await attempt('tampered cookie', {
  cookie: cookie.replace('session-1', 'session-2'),
  'x-csrf-token': csrfToken,
});
await app.close();
