import Fastify, { type FastifyError, type FastifyInstance } from 'fastify';
import { type Config, defaultConfig } from './config.ts';
import { WorkerPool } from './pool.ts';
import { runJob } from './runner.ts';
import { fieldErrors, jobRequestSchema } from './schema.ts';
import { errorBody } from './types.ts';

declare module 'fastify' {
  interface FastifyInstance {
    pool: WorkerPool;
  }
}

function parseId(raw: string): number | null {
  return /^\d+$/.test(raw) ? Number(raw) : null;
}

export async function buildApp(overrides: Partial<Config> = {}): Promise<FastifyInstance> {
  const config = { ...defaultConfig, ...overrides };
  const app = Fastify({ loggerInstance: config.logger });
  app.decorate('pool', new WorkerPool(config, runJob, app.log));

  app.addHook('onClose', async () => {
    await app.pool.shutdown();
  });

  app.setNotFoundHandler((_request, reply) =>
    reply.code(404).send(errorBody('not_found', 'route not found')),
  );

  app.setErrorHandler((error: FastifyError, request, reply) => {
    const status = error.statusCode ?? 500;
    if (status >= 500) {
      request.log.error({ err: error }, 'unhandled error');
      return reply.code(500).send(errorBody('internal', 'internal error'));
    }
    const code = status === 413 ? 'payload_too_large' : 'bad_request';
    return reply.code(status).send(errorBody(code, error.message));
  });

  app.post('/jobs', async (request, reply) => {
    const parsed = jobRequestSchema.safeParse(request.body);
    if (!parsed.success) {
      return reply
        .code(422)
        .send(errorBody('validation_failed', 'invalid job', fieldErrors(parsed.error)));
    }
    const submitted = app.pool.submit(parsed.data);
    if (submitted.ok) {
      return reply.code(202).send({ id: submitted.job.id });
    }
    if (submitted.reason === 'full') {
      return reply
        .code(429)
        .header('Retry-After', '1')
        .send(errorBody('rate_limited', 'job queue is full'));
    }
    return reply.code(503).send(errorBody('unavailable', 'shutting down'));
  });

  app.get<{ Params: { id: string } }>('/jobs/:id', async (request, reply) => {
    const id = parseId(request.params.id);
    const job = id === null ? undefined : app.pool.get(id);
    if (!job) {
      return reply.code(404).send(errorBody('not_found', 'job not found'));
    }
    return job;
  });

  app.get('/jobs', async () => {
    const items = app.pool.list();
    return { items, total: items.length };
  });

  app.delete<{ Params: { id: string } }>('/jobs/:id', async (request, reply) => {
    const id = parseId(request.params.id);
    const outcome = id === null ? 'not_found' : app.pool.cancel(id);
    if (outcome === 'not_found') {
      return reply.code(404).send(errorBody('not_found', 'job not found'));
    }
    if (outcome === 'finished') {
      return reply.code(409).send(errorBody('conflict', 'job already finished'));
    }
    return reply.code(204).send();
  });

  return app;
}
