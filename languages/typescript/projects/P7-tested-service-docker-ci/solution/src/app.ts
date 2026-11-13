import Fastify, { type FastifyError, type FastifyInstance } from 'fastify';
import type { LogLevel } from './config.ts';
import { Metrics } from './metrics.ts';
import { Readiness } from './readiness.ts';
import { fieldErrors, newTaskSchema, pageSchema } from './schema.ts';
import { InMemoryTaskStore } from './store.ts';
import { errorBody, type TaskStore } from './types.ts';

declare module 'fastify' {
  interface FastifyInstance {
    readiness: Readiness;
    metrics: Metrics;
  }
}

export interface AppOptions {
  store?: TaskStore;
  /** Injected so tests can freeze `created_at`. */
  now?: () => Date;
  logLevel?: LogLevel | 'silent';
}

const MAX_LIMIT = 100;

export async function buildApp(options: AppOptions = {}): Promise<FastifyInstance> {
  const { store = new InMemoryTaskStore(), now = () => new Date(), logLevel = 'silent' } = options;
  const app = Fastify({ logger: { level: logLevel } });
  app.decorate('readiness', new Readiness());
  app.decorate('metrics', new Metrics());

  app.setNotFoundHandler((_request, reply) =>
    reply.code(404).send(errorBody('not_found', 'route not found')),
  );

  app.setErrorHandler((error: FastifyError, request, reply) => {
    const status = error.statusCode ?? 500;
    if (status >= 500) {
      request.log.error({ err: error }, 'unhandled error');
      return reply.code(500).send(errorBody('internal', 'internal error'));
    }
    return reply.code(status).send(errorBody('bad_request', error.message));
  });

  app.addHook('onRequest', async () => {
    app.metrics.countRequest();
  });

  app.get('/healthz', async () => ({ status: 'ok' }));

  app.get('/readyz', async (_request, reply) =>
    app.readiness.isReady ? { status: 'ready' } : reply.code(503).send({ status: 'not_ready' }),
  );

  app.get('/metrics', async (_request, reply) =>
    reply.type('text/plain; charset=utf-8').send(app.metrics.render()),
  );

  app.post('/tasks', async (request, reply) => {
    const parsed = newTaskSchema.safeParse(request.body);
    if (!parsed.success) {
      return reply
        .code(422)
        .send(errorBody('validation_failed', 'invalid task', fieldErrors(parsed.error)));
    }
    const createdAt = now()
      .toISOString()
      .replace(/\.\d{3}Z$/, 'Z');
    const task = store.create({ ...parsed.data, created_at: createdAt });
    return reply.code(201).header('Location', `/tasks/${task.id}`).send(task);
  });

  app.get('/tasks', async (request, reply) => {
    const page = pageSchema.safeParse(request.query);
    if (!page.success) {
      return reply.code(400).send(errorBody('bad_request', 'invalid limit or offset'));
    }
    const limit = Math.min(page.data.limit, MAX_LIMIT);
    const { items, total } = store.list(limit, page.data.offset);
    return { items, total, limit, offset: page.data.offset };
  });

  app.get<{ Params: { id: string } }>('/tasks/:id', async (request, reply) => {
    const task = /^\d+$/.test(request.params.id) ? store.get(Number(request.params.id)) : undefined;
    if (!task) {
      return reply.code(404).send(errorBody('not_found', 'task not found'));
    }
    return task;
  });

  return app;
}
