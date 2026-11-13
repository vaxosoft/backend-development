import type { FastifyInstance, FastifyRequest } from 'fastify';
import type { Deps } from './types.ts';

/**
 * TODO(P5): read `Authorization: Bearer <token>`, verify it, store the id in `request.userId`
 * and return it. Anything missing or invalid is `401 unauthorized`.
 */
export async function requireUser(_request: FastifyRequest, _secret: string): Promise<number> {
  throw new Error('TODO');
}

/**
 * TODO(P5): implement the handlers (see the README for the full rules). Use `parseBody` with the
 * schemas from schemas.ts for validation and throw `HttpError` for expected failures.
 */
export async function registerRoutes(app: FastifyInstance, _deps: Deps): Promise<void> {
  // POST /auth/register -> 201 {id, email}; duplicate email -> 409 conflict
  app.post('/auth/register', async () => {
    throw new Error('TODO');
  });

  // POST /auth/login -> 200 {token}; same 401 for unknown user and wrong password
  app.post('/auth/login', async () => {
    throw new Error('TODO');
  });

  // POST /links (auth, rate limited) -> 201 {slug, url}; taken slug -> 409 conflict
  app.post('/links', async () => {
    throw new Error('TODO');
  });

  // GET /links (auth) -> {items, total, limit, offset} with the caller's own links
  app.get('/links', async () => {
    throw new Error('TODO');
  });

  // DELETE /links/:slug (auth, owner only) -> 204, other user -> 403, unknown -> 404
  app.delete('/links/:slug', async () => {
    throw new Error('TODO');
  });

  // GET /:slug (public) -> 302 Location and a counted hit; unknown -> 404
  app.get('/:slug', async () => {
    throw new Error('TODO');
  });
}
