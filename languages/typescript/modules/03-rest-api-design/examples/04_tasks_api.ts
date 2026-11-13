// The Task Tracker REST API: resources, verbs, status codes, validation, filtering, idempotency, versioning.
import Fastify, { type FastifyPluginAsync } from 'fastify';
import { closeOnSignal, listenAddress } from '../../02-http-server/examples/_shared.ts';
import {
  HttpError,
  listTasks,
  notFound,
  parseListQuery,
  parseOr422,
  registerErrorHandling,
  type Task,
  taskInputSchema,
  taskPatchSchema,
} from './_shared.ts';

const tasks = new Map<number, Task>();
let lastId = 0;

// A counting clock keeps the demo output deterministic: 09:00:01, 09:00:02, ...
const now = (): string =>
  new Date(Date.UTC(2030, 0, 1, 9, 0, lastId)).toISOString().replace('.000Z', 'Z');

const idempotency = new Map<string, { fingerprint: string; task: Task }>();

function findTask(rawId: string): Task {
  const task = /^\d+$/.test(rawId) ? tasks.get(Number(rawId)) : undefined;
  if (!task) throw notFound(`task ${rawId}`);
  return task;
}

const v1: FastifyPluginAsync = async (app) => {
  app.post('/tasks', async (req, reply) => {
    const input = parseOr422(taskInputSchema, req.body, 'task');

    const key = req.headers['idempotency-key'];
    const fingerprint = JSON.stringify(input);
    if (typeof key === 'string') {
      const seen = idempotency.get(key);
      if (seen && seen.fingerprint !== fingerprint) {
        throw new HttpError(
          409,
          'conflict',
          'Idempotency-Key was already used with a different body',
        );
      }
      if (seen) {
        return reply
          .code(201)
          .header('location', `/v1/tasks/${seen.task.id}`)
          .header('idempotent-replayed', 'true')
          .send(seen.task);
      }
    }

    lastId += 1;
    const task: Task = { id: lastId, ...input, created_at: now() };
    tasks.set(task.id, task);
    if (typeof key === 'string') idempotency.set(key, { fingerprint, task: structuredClone(task) });
    return reply.code(201).header('location', `/v1/tasks/${task.id}`).send(task);
  });

  app.get('/tasks', async (req) => listTasks([...tasks.values()], parseListQuery(req.query)));

  app.get<{ Params: { id: string } }>('/tasks/:id', async (req) => findTask(req.params.id));

  // PUT replaces the whole resource: omitted optional fields go back to their defaults.
  app.put<{ Params: { id: string } }>('/tasks/:id', async (req) => {
    const current = findTask(req.params.id);
    const replacement: Task = {
      id: current.id,
      ...parseOr422(taskInputSchema, req.body, 'task'),
      created_at: current.created_at,
    };
    tasks.set(current.id, replacement);
    return replacement;
  });

  // PATCH changes only the fields that are present; `due_date: null` clears the date.
  app.patch<{ Params: { id: string } }>('/tasks/:id', async (req) => {
    const current = findTask(req.params.id);
    const patch = parseOr422(taskPatchSchema, req.body, 'task');
    const updated: Task = { ...current };
    if (patch.title !== undefined) updated.title = patch.title;
    if (patch.status !== undefined) updated.status = patch.status;
    if (patch.due_date !== undefined) updated.due_date = patch.due_date;
    if (patch.tags !== undefined) updated.tags = patch.tags;
    tasks.set(current.id, updated);
    return updated;
  });

  app.delete<{ Params: { id: string } }>('/tasks/:id', async (req, reply) => {
    tasks.delete(findTask(req.params.id).id);
    return reply.code(204).send();
  });
};

const app = Fastify();
registerErrorHandling(app);
app.get('/healthz', async () => ({ status: 'ok' }));
await app.register(v1, { prefix: '/v1' }); // a /v2 plugin could be registered next to it later

const { host, port } = listenAddress();
await app.listen({ host, port });
console.log('listening');

closeOnSignal(() => app.close());
