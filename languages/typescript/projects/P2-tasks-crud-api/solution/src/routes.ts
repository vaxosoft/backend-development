import type { FastifyInstance } from 'fastify';
import { badRequest, notFound, validationFailed } from './errors.ts';
import type { TaskRepository } from './repository.ts';
import { parseListQuery, parseTaskInput, parseTaskPatch } from './validation.ts';

/** Path ids that are not positive integers cannot exist, so they are plain 404s. */
export function parseId(raw: string): number {
  if (!/^[1-9]\d{0,14}$/.test(raw)) throw notFound('task');
  return Number(raw);
}

export function registerTaskRoutes(app: FastifyInstance, repo: TaskRepository): void {
  app.post('/tasks', async (req, reply) => {
    const parsed = parseTaskInput(req.body);
    if (!parsed.ok) throw validationFailed(parsed.fields);
    const task = repo.create(parsed.value);
    return reply.code(201).header('location', `/tasks/${task.id}`).send(task);
  });

  app.get('/tasks', async (req) => {
    const parsed = parseListQuery(req.query);
    if (!parsed.ok) {
      const detail = Object.entries(parsed.fields).map(([field, message]) => `${field} ${message}`);
      throw badRequest(`invalid query: ${detail.join('; ')}`);
    }
    const { items, total } = repo.list(parsed.value);
    return { items, total, limit: parsed.value.limit, offset: parsed.value.offset };
  });

  app.get<{ Params: { id: string } }>('/tasks/:id', async (req) => {
    const task = repo.get(parseId(req.params.id));
    if (!task) throw notFound('task');
    return task;
  });

  app.put<{ Params: { id: string } }>('/tasks/:id', async (req) => {
    const id = parseId(req.params.id);
    const parsed = parseTaskInput(req.body);
    if (!parsed.ok) throw validationFailed(parsed.fields);
    const task = repo.replace(id, parsed.value);
    if (!task) throw notFound('task');
    return task;
  });

  app.patch<{ Params: { id: string } }>('/tasks/:id', async (req) => {
    const id = parseId(req.params.id);
    const parsed = parseTaskPatch(req.body);
    if (!parsed.ok) throw validationFailed(parsed.fields);
    const task = repo.patch(id, parsed.value);
    if (!task) throw notFound('task');
    return task;
  });

  app.delete<{ Params: { id: string } }>('/tasks/:id', async (req, reply) => {
    if (!repo.remove(parseId(req.params.id))) throw notFound('task');
    return reply.code(204).send();
  });
}
