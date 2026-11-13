import type { FastifyInstance } from 'fastify';
import { notFound } from './errors.ts';
import type { TaskRepository } from './repository.ts';

/** Path ids that are not positive integers cannot exist, so they are plain 404s. */
export function parseId(raw: string): number {
  if (!/^[1-9]\d{0,14}$/.test(raw)) throw notFound('task');
  return Number(raw);
}

export function registerTaskRoutes(app: FastifyInstance, repo: TaskRepository): void {
  app.get<{ Params: { id: string } }>('/tasks/:id', async (req) => {
    const task = repo.get(parseId(req.params.id));
    if (!task) throw notFound('task');
    return task;
  });

  // TODO: POST /tasks    -> 201 + Location, validate with parseTaskInput (422 on failure)
  // TODO: GET /tasks     -> parseListQuery (400 on failure), reply { items, total, limit, offset }
  // TODO: PUT /tasks/:id -> full replace
  // TODO: PATCH /tasks/:id -> partial update
  // TODO: DELETE /tasks/:id -> 204
}
