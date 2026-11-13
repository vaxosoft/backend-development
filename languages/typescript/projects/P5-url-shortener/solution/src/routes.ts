import { randomInt } from 'node:crypto';
import type { FastifyInstance, FastifyRequest } from 'fastify';
import { HttpError, parseBody } from './errors.ts';
import { hashPassword, verifyPassword } from './passwords.ts';
import { createLinkSchema, loginSchema, registerSchema } from './schemas.ts';
import { signToken, verifyToken } from './tokens.ts';
import type { Deps } from './types.ts';

const ALPHABET = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
// GET /links is a static route and always wins over GET /:slug, so this slug could never redirect.
const RESERVED_SLUGS = new Set(['links']);
const DEFAULT_LIMIT = 20;
const MAX_LIMIT = 100;

function generateSlug(): string {
  return Array.from({ length: 7 }, () => ALPHABET.charAt(randomInt(ALPHABET.length))).join('');
}

function parsePaging(query: { limit?: string; offset?: string }) {
  const read = (raw: string | undefined, fallback: number, min: number) => {
    if (raw === undefined) {
      return fallback;
    }
    const value = Number(raw);
    if (raw.trim() === '' || !Number.isInteger(value) || value < min) {
      throw new HttpError(400, 'bad_request', 'limit and offset must be non-negative integers');
    }
    return value;
  };
  return {
    limit: Math.min(read(query.limit, DEFAULT_LIMIT, 1), MAX_LIMIT),
    offset: read(query.offset, 0, 0),
  };
}

export async function requireUser(request: FastifyRequest, secret: string): Promise<number> {
  const [scheme, token] = (request.headers.authorization ?? '').split(' ');
  const userId =
    scheme?.toLowerCase() === 'bearer' && token ? await verifyToken(token, secret) : null;
  if (userId === null) {
    throw new HttpError(401, 'unauthorized', 'missing or invalid token');
  }
  request.userId = userId;
  return userId;
}

export async function registerRoutes(app: FastifyInstance, deps: Deps): Promise<void> {
  const { config, store, limiter } = deps;
  // Unknown users are verified against this hash so they cost the same time as wrong passwords.
  const decoyHash = await hashPassword('decoy-password', config.scryptCost);

  app.post('/auth/register', async (request, reply) => {
    const { email, password } = parseBody(registerSchema, request.body);
    const user = store.createUser(email, await hashPassword(password, config.scryptCost));
    if (!user) {
      throw new HttpError(409, 'conflict', 'email already registered');
    }
    return reply.code(201).send({ id: user.id, email: user.email });
  });

  app.post('/auth/login', async (request) => {
    const { email, password } = parseBody(loginSchema, request.body);
    const user = store.findUserByEmail(email);
    const passwordOk = await verifyPassword(password, user?.passwordHash ?? decoyHash);
    if (!user || !passwordOk) {
      throw new HttpError(401, 'unauthorized', 'invalid email or password');
    }
    return { token: await signToken(user.id, config.jwtSecret) };
  });

  app.post('/links', async (request, reply) => {
    const userId = await requireUser(request, config.jwtSecret);
    // Every attempt costs a token, valid or not.
    const decision = limiter.take(String(userId));
    if (!decision.allowed) {
      throw new HttpError(429, 'rate_limited', 'too many links created, slow down', {
        headers: { 'retry-after': String(decision.retryAfterSeconds) },
      });
    }
    const { url, slug } = parseBody(createLinkSchema, request.body);

    if (slug !== undefined) {
      if (RESERVED_SLUGS.has(slug) || !store.createLink(slug, url, userId)) {
        throw new HttpError(409, 'conflict', 'slug already taken');
      }
      return reply.code(201).send({ slug, url });
    }
    for (let attempt = 0; attempt < 5; attempt++) {
      const generated = generateSlug();
      if (store.createLink(generated, url, userId)) {
        return reply.code(201).send({ slug: generated, url });
      }
    }
    throw new Error('could not generate a free slug');
  });

  app.get<{ Querystring: { limit?: string; offset?: string } }>('/links', async (request) => {
    const userId = await requireUser(request, config.jwtSecret);
    const { limit, offset } = parsePaging(request.query);
    const { items, total } = store.listLinks(userId, limit, offset);
    return {
      items: items.map((l) => ({
        slug: l.slug,
        url: l.url,
        hits: l.hits,
        created_at: l.createdAt,
      })),
      total,
      limit,
      offset,
    };
  });

  app.delete<{ Params: { slug: string } }>('/links/:slug', async (request, reply) => {
    const userId = await requireUser(request, config.jwtSecret);
    const link = store.findLink(request.params.slug);
    if (!link) {
      throw new HttpError(404, 'not_found', 'link not found');
    }
    if (link.userId !== userId) {
      throw new HttpError(403, 'forbidden', 'you do not own this link');
    }
    store.deleteLink(link.slug);
    return reply.code(204).send();
  });

  app.get<{ Params: { slug: string } }>('/:slug', async (request, reply) => {
    const url = store.hit(request.params.slug);
    if (url === null) {
      throw new HttpError(404, 'not_found', 'link not found');
    }
    return reply.redirect(url, 302);
  });
}
