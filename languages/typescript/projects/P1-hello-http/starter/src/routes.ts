import type { FastifyInstance } from 'fastify';

export const MAX_SLOW_MS = 2000;

export function registerRoutes(app: FastifyInstance): void {
  app.get('/healthz', async () => ({ status: 'ok' }));

  // TODO: GET /hello and GET /hello/:name (text/plain)
  // TODO: GET /slow?ms=N (clamp to MAX_SLOW_MS, 400 bad_request for a bad value)
  // TODO: GET /panic (throws)
}
