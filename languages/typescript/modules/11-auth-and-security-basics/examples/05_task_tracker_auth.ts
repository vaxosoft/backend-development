import { randomBytes } from 'node:crypto';
import Fastify, { type FastifyReply, type FastifyRequest } from 'fastify';
import { errors, jwtVerify, SignJWT } from 'jose';
import { z } from 'zod';
import { hashPassword, TokenBucketLimiter, verifyPassword } from './_shared.ts';

type Role = 'admin' | 'member';

interface AuthUser {
  id: number;
  role: Role;
}

declare module 'fastify' {
  interface FastifyRequest {
    user: AuthUser | null;
  }
}

const ISSUER = 'task-tracker';
// Never hard-code a secret. Without JWT_SECRET we make a random one, so tokens die with the process.
const secret = new TextEncoder().encode(process.env.JWT_SECRET ?? randomBytes(32).toString('hex'));

const users = [
  {
    id: 1,
    email: 'alice@example.com',
    role: 'admin' as const,
    hash: await hashPassword('alice-pass-1'),
  },
  {
    id: 2,
    email: 'bob@example.com',
    role: 'member' as const,
    hash: await hashPassword('bob-pass-12'),
  },
];
const decoyHash = await hashPassword('decoy');

const errorBody = (code: string, message: string) => ({ error: { code, message } });
const Login = z.object({ email: z.string(), password: z.string() });
const NewTask = z.object({ title: z.string().trim().min(1).max(200) });
const limiter = new TokenBucketLimiter({ burst: 3, refillPerSecond: 1 });

const app = Fastify();
app.decorateRequest('user', null);

app.post('/auth/login', async (req, reply) => {
  const body = Login.safeParse(req.body);
  if (!body.success)
    return reply.code(400).send(errorBody('bad_request', 'email and password required'));
  const user = users.find((u) => u.email === body.data.email);
  // Verify against a decoy hash for unknown users so both failures cost the same time and say the same thing.
  const ok = await verifyPassword(body.data.password, user?.hash ?? decoyHash);
  if (!user || !ok) return reply.code(401).send(errorBody('unauthorized', 'invalid credentials'));

  const token = await new SignJWT({ role: user.role })
    .setProtectedHeader({ alg: 'HS256' })
    .setSubject(String(user.id))
    .setIssuer(ISSUER)
    .setIssuedAt()
    .setExpirationTime('1h')
    .sign(secret);
  return { token };
});

async function authenticate(req: FastifyRequest, reply: FastifyReply) {
  const header = req.headers.authorization ?? '';
  const token = header.startsWith('Bearer ') ? header.slice('Bearer '.length) : '';
  try {
    const { payload } = await jwtVerify(token, secret, { algorithms: ['HS256'], issuer: ISSUER });
    req.user = { id: Number(payload.sub), role: payload.role === 'admin' ? 'admin' : 'member' };
  } catch (err) {
    if (!(err instanceof errors.JOSEError) && !(err instanceof TypeError)) throw err;
    return reply
      .code(401)
      .header('www-authenticate', 'Bearer')
      .send(errorBody('unauthorized', 'missing or invalid token'));
  }
}

const requireRole = (role: Role) => async (req: FastifyRequest, reply: FastifyReply) => {
  if (req.user?.role !== role)
    return reply.code(403).send(errorBody('forbidden', `requires role ${role}`));
};

const tasks = new Map<number, { id: number; title: string; owner_id: number }>();

app.get('/tasks', { preHandler: authenticate }, async (req) => ({
  items: [...tasks.values()].filter((t) => t.owner_id === req.user?.id),
}));

app.post('/tasks', { preHandler: authenticate }, async (req, reply) => {
  const verdict = limiter.take(`user:${req.user?.id}`);
  if (!verdict.allowed) {
    return reply
      .code(429)
      .header('retry-after', String(verdict.retryAfterSeconds))
      .send(errorBody('rate_limited', 'too many requests'));
  }
  const body = NewTask.safeParse(req.body);
  if (!body.success) return reply.code(422).send(errorBody('validation_failed', 'invalid task'));
  const task = { id: tasks.size + 1, title: body.data.title, owner_id: req.user?.id ?? 0 };
  tasks.set(task.id, task);
  return reply.code(201).send(task);
});

app.delete(
  '/tasks/:id',
  { preHandler: [authenticate, requireRole('admin')] },
  async (req, reply) => {
    const id = Number((req.params as { id: string }).id);
    return tasks.delete(id)
      ? reply.code(204).send()
      : reply.code(404).send(errorBody('not_found', 'task not found'));
  },
);

process.once('SIGINT', () => void app.close());
process.once('SIGTERM', () => void app.close());

const [host = '127.0.0.1', port = '8080'] = (process.env.ADDR ?? '127.0.0.1:8080').split(':');
await app.listen({ host, port: Number(port) });
console.log(`listening on ${host}:${port}`);
