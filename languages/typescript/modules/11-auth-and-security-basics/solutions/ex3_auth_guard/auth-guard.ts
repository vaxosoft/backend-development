import Fastify, { type FastifyInstance, type FastifyReply, type FastifyRequest } from 'fastify';
import { z } from 'zod';

export type Role = 'admin' | 'member';

export interface Claims {
  sub: number;
  role: Role;
}

export type Take = { allowed: true } | { allowed: false; retryAfterSeconds: number };

export interface Deps {
  /** Resolves to the claims of a valid token and to `null` for any invalid one. May reject: treat that as invalid. */
  verifyToken(token: string): Promise<Claims | null>;
  /** One bucket per key; `take` consumes one unit. */
  limiter: { take(key: string): Take };
}

declare module 'fastify' {
  interface FastifyRequest {
    claims: Claims | null;
  }
}

const NewTask = z.object({ title: z.string().trim().min(1).max(200) });
const envelope = (code: string, message: string) => ({ error: { code, message } });

/**
 * Builds the secured Task Tracker API. Every error body is `{"error":{"code","message"}}`.
 *
 * Authentication (all routes below except `/healthz`):
 * - credentials come from `Authorization: Bearer <token>` (scheme matched case-insensitively);
 *   a missing header, another scheme (`Basic ...`), an empty token, an invalid token or a
 *   `verifyToken` failure -> 401 `unauthorized` with a `WWW-Authenticate: Bearer` header.
 *
 * Routes:
 * - `GET /healthz` -> 200 `{"status":"ok"}`, no authentication.
 * - `GET /me` -> 200 `{"id": <sub>, "role": <role>}`.
 * - `GET /admin/stats` -> 200 `{"users": 2}` for admins; members get 403 `forbidden`.
 * - `POST /tasks` -> 201 `{"id":1,"title":..., "owner_id": <sub>}`. Rate limited per user with
 *   `limiter.take("user:<sub>")`: when not allowed, 429 `rate_limited` with `Retry-After: <seconds>`.
 *   The limiter is consulted only for authenticated users, and before the body is validated.
 *   The body needs a non-empty `title` (after trimming, at most 200 characters), else 422 `validation_failed`.
 *
 * Every response (including errors) carries `X-Content-Type-Options: nosniff`;
 * responses of authenticated routes also carry `Cache-Control: no-store`.
 */
export function buildApp({ verifyToken, limiter }: Deps): FastifyInstance {
  const app = Fastify();
  app.decorateRequest('claims', null);

  app.addHook('onSend', async (_req, reply) => {
    reply.header('x-content-type-options', 'nosniff');
  });

  async function authenticate(req: FastifyRequest, reply: FastifyReply) {
    const match = /^bearer +(\S+)$/i.exec(req.headers.authorization ?? '');
    let claims: Claims | null = null;
    if (match?.[1]) {
      try {
        claims = await verifyToken(match[1]);
      } catch {
        claims = null;
      }
    }
    if (!claims) {
      return reply
        .code(401)
        .header('www-authenticate', 'Bearer')
        .header('cache-control', 'no-store')
        .send(envelope('unauthorized', 'missing or invalid token'));
    }
    req.claims = claims;
    reply.header('cache-control', 'no-store');
  }

  const requireAdmin = async (req: FastifyRequest, reply: FastifyReply) => {
    if (req.claims?.role !== 'admin') {
      return reply.code(403).send(envelope('forbidden', 'requires role admin'));
    }
  };

  app.get('/healthz', async () => ({ status: 'ok' }));

  app.get('/me', { preHandler: authenticate }, async (req) => ({
    id: req.claims?.sub,
    role: req.claims?.role,
  }));

  app.get('/admin/stats', { preHandler: [authenticate, requireAdmin] }, async () => ({ users: 2 }));

  let nextId = 1;
  app.post('/tasks', { preHandler: authenticate }, async (req, reply) => {
    const owner = req.claims?.sub ?? 0;
    const verdict = limiter.take(`user:${owner}`);
    if (!verdict.allowed) {
      return reply
        .code(429)
        .header('retry-after', String(verdict.retryAfterSeconds))
        .send(envelope('rate_limited', 'too many requests'));
    }
    const body = NewTask.safeParse(req.body);
    if (!body.success) return reply.code(422).send(envelope('validation_failed', 'invalid task'));
    return reply.code(201).send({ id: nextId++, title: body.data.title, owner_id: owner });
  });

  return app;
}
