// Code under test for the examples of this module. Not runnable.
import Fastify, { type FastifyInstance } from 'fastify';
import { z } from 'zod';

export type Status = 'todo' | 'in_progress' | 'done';

export interface Task {
  id: number;
  title: string;
  status: Status;
  due_date: string | null;
  tags: string[];
  owner_id: number | null;
  created_at: string;
}

export interface NewTask {
  title: string;
  status?: Status;
  due_date?: string | null;
  tags?: string[];
  owner_id?: number | null;
}

export interface ListOptions {
  status?: Status;
  limit?: number;
  offset?: number;
}

/** Storage port. Implemented by an in-memory store here and by SQLite in `_sqlite-repo.ts`. */
export interface TaskRepo {
  create(input: NewTask): Task;
  get(id: number): Task | undefined;
  /** Newest first (highest id first). `limit` defaults to 20 and is clamped to 1-100. */
  list(options?: ListOptions): { items: Task[]; total: number };
  /** Only the fields present in `patch` change; `due_date: null` clears the date. */
  update(id: number, patch: Partial<NewTask>): Task | undefined;
  delete(id: number): boolean;
}

export type Clock = () => Date;

export class NotFoundError extends Error {
  constructor(what: string) {
    super(`${what} not found`);
    this.name = 'NotFoundError';
  }
}

export class ValidationError extends Error {
  readonly fields: Record<string, string>;

  constructor(fields: Record<string, string>) {
    super('validation failed');
    this.name = 'ValidationError';
    this.fields = fields;
  }
}

/** Trims, lowercases, drops empties and duplicates (keeping first-seen order); at most 10 tags of 1-30 chars. */
export function normalizeTags(tags: readonly string[]): string[] {
  const seen = new Set<string>();
  for (const raw of tags) {
    const tag = raw.trim().toLowerCase();
    if (tag === '') continue;
    if (tag.length > 30) throw new ValidationError({ tags: 'each tag must be 1-30 characters' });
    seen.add(tag);
  }
  if (seen.size > 10) throw new ValidationError({ tags: 'at most 10 tags' });
  return [...seen];
}

/** Returns the date when `text` is a real calendar date `YYYY-MM-DD`, otherwise `null`. */
export function parseDueDate(text: string): string | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(text);
  if (!match) return null;
  const [year, month, day] = [Number(match[1]), Number(match[2]), Number(match[3])];
  const date = new Date(Date.UTC(year, month - 1, day));
  const real =
    date.getUTCFullYear() === year && date.getUTCMonth() === month - 1 && date.getUTCDate() === day;
  return real ? text : null;
}

/** Calls `fn` once, `waitMs` after the last call (e.g. to coalesce rapid saves into one write). */
export function debounce<A extends unknown[]>(fn: (...args: A) => void, waitMs: number) {
  let timer: ReturnType<typeof setTimeout> | undefined;
  return (...args: A): void => {
    clearTimeout(timer);
    timer = setTimeout(() => fn(...args), waitMs);
  };
}

const toTimestamp = (date: Date) => date.toISOString().replace(/\.\d{3}Z$/, 'Z');

export function createMemoryRepo(clock: Clock = () => new Date()): TaskRepo {
  const tasks = new Map<number, Task>();
  let nextId = 1;

  return {
    create(input) {
      const task: Task = {
        id: nextId++,
        title: input.title,
        status: input.status ?? 'todo',
        due_date: input.due_date ?? null,
        tags: [...(input.tags ?? [])],
        owner_id: input.owner_id ?? null,
        created_at: toTimestamp(clock()),
      };
      tasks.set(task.id, task);
      return structuredClone(task);
    },

    get(id) {
      const task = tasks.get(id);
      return task && structuredClone(task);
    },

    list({ status, limit = 20, offset = 0 } = {}) {
      const matching = [...tasks.values()]
        .filter((t) => status === undefined || t.status === status)
        .sort((a, b) => b.id - a.id);
      const size = Math.min(Math.max(limit, 1), 100);
      return {
        items: structuredClone(matching.slice(offset, offset + size)),
        total: matching.length,
      };
    },

    update(id, patch) {
      const task = tasks.get(id);
      if (!task) return undefined;
      if (patch.title !== undefined) task.title = patch.title;
      if (patch.status !== undefined) task.status = patch.status;
      if (patch.due_date !== undefined) task.due_date = patch.due_date;
      if (patch.tags !== undefined) task.tags = [...patch.tags];
      return structuredClone(task);
    },

    delete(id) {
      return tasks.delete(id);
    },
  };
}

export interface Notifier {
  taskCompleted(task: Task): Promise<void>;
}

export interface ServiceDeps {
  repo: TaskRepo;
  notifier: Notifier;
  clock: Clock;
}

/** Business rules on top of a repo; every collaborator is injected so tests can replace it. */
export class TaskService {
  readonly #deps: ServiceDeps;

  constructor(deps: ServiceDeps) {
    this.#deps = deps;
  }

  /** Marks the task done and notifies once; completing a done task is a no-op. */
  async complete(id: number): Promise<Task> {
    const { repo, notifier } = this.#deps;
    const task = repo.get(id);
    if (!task) throw new NotFoundError(`task ${id}`);
    if (task.status === 'done') return task;
    const updated = repo.update(id, { status: 'done' });
    if (!updated) throw new NotFoundError(`task ${id}`);
    await notifier.taskCompleted(updated);
    return updated;
  }

  /** Open tasks whose due date is before today (UTC) according to the injected clock. */
  overdue(): Task[] {
    const today = this.#deps.clock().toISOString().slice(0, 10);
    return this.#deps.repo
      .list({ limit: 100 })
      .items.filter((t) => t.status !== 'done' && t.due_date !== null && t.due_date < today);
  }
}

const envelope = (code: string, message: string, fields?: Record<string, string>) => ({
  error: { code, message, ...(fields ? { fields } : {}) },
});

const TaskInput = z.object({
  title: z.string().trim().min(1).max(200),
  status: z.enum(['todo', 'in_progress', 'done']).optional(),
  due_date: z.string().nullable().optional(),
  tags: z.array(z.string()).optional(),
});

/** A small Task Tracker HTTP API over any `TaskRepo`. */
export function buildApp(repo: TaskRepo): FastifyInstance {
  const app = Fastify();

  app.setErrorHandler((err: Error & { statusCode?: number }, _req, reply) => {
    if (err instanceof ValidationError) {
      return reply.code(422).send(envelope('validation_failed', 'invalid task', err.fields));
    }
    if (err.statusCode === 400)
      return reply.code(400).send(envelope('bad_request', 'malformed request'));
    return reply.code(500).send(envelope('internal', 'internal server error'));
  });

  app.post('/tasks', async (req, reply) => {
    const parsed = TaskInput.safeParse(req.body);
    if (!parsed.success) {
      throw new ValidationError({ title: 'must be 1-200 characters' });
    }
    const { due_date, tags, ...rest } = parsed.data;
    if (due_date && parseDueDate(due_date) === null) {
      throw new ValidationError({ due_date: 'must be a date like 2030-01-31' });
    }
    const task = repo.create({
      ...rest,
      due_date: due_date ?? null,
      ...(tags ? { tags: normalizeTags(tags) } : {}),
    });
    return reply.code(201).header('location', `/tasks/${task.id}`).send(task);
  });

  app.get('/tasks/:id', async (req, reply) => {
    const task = repo.get(Number((req.params as { id: string }).id));
    return task ?? reply.code(404).send(envelope('not_found', 'task not found'));
  });

  app.get('/tasks', async (req) => {
    const query = req.query as { status?: Status; limit?: string; offset?: string };
    const { items, total } = repo.list({
      ...(query.status ? { status: query.status } : {}),
      ...(query.limit ? { limit: Number(query.limit) } : {}),
      ...(query.offset ? { offset: Number(query.offset) } : {}),
    });
    return {
      items,
      total,
      limit: Math.min(Math.max(Number(query.limit ?? 20), 1), 100),
      offset: Number(query.offset ?? 0),
    };
  });

  app.delete('/tasks/:id', async (req, reply) => {
    const removed = repo.delete(Number((req.params as { id: string }).id));
    return removed
      ? reply.code(204).send()
      : reply.code(404).send(envelope('not_found', 'task not found'));
  });

  return app;
}
