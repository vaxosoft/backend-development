import Fastify, { type FastifyInstance } from 'fastify';
import type { z } from 'zod';
import {
  listQuerySchema,
  TaskStore,
  taskInputSchema,
  taskPatchSchema,
  zodFields,
} from './task-model.ts';

type ErrorCode =
  | 'bad_request'
  | 'not_found'
  | 'conflict'
  | 'validation_failed'
  | 'payload_too_large'
  | 'internal';

class ApiError extends Error {
  readonly statusCode: number;
  readonly code: ErrorCode;
  readonly fields: Record<string, string> | undefined;

  constructor(
    statusCode: number,
    code: ErrorCode,
    message: string,
    fields?: Record<string, string>,
  ) {
    super(message);
    this.statusCode = statusCode;
    this.code = code;
    this.fields = fields;
  }
}

function envelope(code: ErrorCode, message: string, fields?: Record<string, string>) {
  return { error: { code, message, ...(fields ? { fields } : {}) } };
}

function statusOf(error: unknown): number | undefined {
  if (typeof error === 'object' && error !== null && 'statusCode' in error) {
    return typeof error.statusCode === 'number' ? error.statusCode : undefined;
  }
  return undefined;
}

function parseBody<S extends z.ZodType>(schema: S, body: unknown): z.output<S> {
  const result = schema.safeParse(body);
  if (!result.success) {
    throw new ApiError(422, 'validation_failed', 'invalid task', zodFields(result.error));
  }
  return result.data;
}

function parseId(raw: string): number {
  if (!/^\d+$/.test(raw)) throw new ApiError(404, 'not_found', `task ${raw} not found`);
  return Number(raw);
}

export function buildApi(store: TaskStore = new TaskStore()): FastifyInstance {
  const app = Fastify({ bodyLimit: 1024 });
  const seenKeys = new Map<string, { fingerprint: string; id: number }>();

  app.setNotFoundHandler(async (req, reply) =>
    reply.code(404).send(envelope('not_found', `route ${req.method} ${req.url} not found`)),
  );

  app.setErrorHandler(async (error, _req, reply) => {
    if (error instanceof ApiError) {
      return reply.code(error.statusCode).send(envelope(error.code, error.message, error.fields));
    }
    const status = statusOf(error);
    if (status === 413)
      return reply.code(413).send(envelope('payload_too_large', 'payload too large'));
    if (status !== undefined && status >= 400 && status < 500) {
      return reply.code(status).send(envelope('bad_request', 'malformed request'));
    }
    return reply.code(500).send(envelope('internal', 'internal server error'));
  });

  const taskOr404 = <T>(value: T | undefined, id: number): T => {
    if (value === undefined) throw new ApiError(404, 'not_found', `task ${id} not found`);
    return value;
  };

  app.post('/tasks', async (req, reply) => {
    const input = parseBody(taskInputSchema, req.body);
    const key = req.headers['idempotency-key'];
    const fingerprint = JSON.stringify(input);

    if (typeof key === 'string') {
      const seen = seenKeys.get(key);
      if (seen && seen.fingerprint !== fingerprint) {
        throw new ApiError(
          409,
          'conflict',
          'Idempotency-Key was already used with a different body',
        );
      }
      const original = seen ? store.get(seen.id) : undefined;
      if (seen && original) {
        return reply
          .code(201)
          .header('location', `/tasks/${seen.id}`)
          .header('idempotent-replayed', 'true')
          .send(original);
      }
    }

    const task = store.create(input);
    if (typeof key === 'string') seenKeys.set(key, { fingerprint, id: task.id });
    return reply.code(201).header('location', `/tasks/${task.id}`).send(task);
  });

  app.get('/tasks', async (req) => {
    const result = listQuerySchema.safeParse(req.query);
    if (!result.success) {
      const [field, message] = Object.entries(zodFields(result.error))[0] ?? ['query', 'invalid'];
      throw new ApiError(400, 'bad_request', `invalid query parameter ${field}: ${message}`);
    }
    return store.list(result.data);
  });

  app.get<{ Params: { id: string } }>('/tasks/:id', async (req) => {
    const id = parseId(req.params.id);
    return taskOr404(store.get(id), id);
  });

  app.put<{ Params: { id: string } }>('/tasks/:id', async (req) => {
    const id = parseId(req.params.id);
    return taskOr404(store.replace(id, parseBody(taskInputSchema, req.body)), id);
  });

  app.patch<{ Params: { id: string } }>('/tasks/:id', async (req) => {
    const id = parseId(req.params.id);
    taskOr404(store.get(id), id);
    return taskOr404(store.patch(id, parseBody(taskPatchSchema, req.body)), id);
  });

  app.delete<{ Params: { id: string } }>('/tasks/:id', async (req, reply) => {
    const id = parseId(req.params.id);
    taskOr404(store.remove(id) ? true : undefined, id);
    return reply.code(204).send();
  });

  return app;
}
