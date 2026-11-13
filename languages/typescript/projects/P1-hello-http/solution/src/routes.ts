import { setTimeout as sleep } from 'node:timers/promises';
import type { FastifyInstance } from 'fastify';
import { errorBody } from './errors.ts';

export const MAX_SLOW_MS = 2000;

export function registerRoutes(app: FastifyInstance): void {
  app.get('/healthz', async () => ({ status: 'ok' }));

  app.get('/hello', async () => 'Hello, world!');

  app.get<{ Params: { name: string } }>(
    '/hello/:name',
    async (req) => `Hello, ${req.params.name}!`,
  );

  app.get<{ Querystring: { ms?: string } }>('/slow', async (req, reply) => {
    const ms = req.query.ms === undefined ? 0 : Number(req.query.ms);
    if (!Number.isInteger(ms) || ms < 0) {
      return reply.code(400).send(errorBody('bad_request', 'ms must be a non-negative integer'));
    }
    await sleep(Math.min(ms, MAX_SLOW_MS));
    return 'done';
  });

  app.get('/panic', async () => {
    throw new Error('boom: this detail must not reach the client');
  });
}
