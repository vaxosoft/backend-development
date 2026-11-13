// Provided building blocks for the exercise: schemas, the in-memory store and a zod-to-fields helper.
// You do not need to change this file.
import { z } from 'zod';

const title = z
  .string('must be a string')
  .trim()
  .min(1, 'must be 1-200 characters')
  .max(200, 'must be 1-200 characters');
const status = z.enum(['todo', 'in_progress', 'done'], 'must be todo, in_progress or done');
const dueDate = z.iso.date('must be a date like 2030-01-31').nullable();
const tags = z
  .array(
    z
      .string('must be a string')
      .min(1, 'must be 1-30 characters')
      .max(30, 'must be 1-30 characters')
      .refine((value) => value === value.toLowerCase(), 'must be lowercase'),
    'must be an array',
  )
  .max(10, 'at most 10 tags')
  .refine((list) => new Set(list).size === list.length, 'must not contain duplicates');

/** Body of POST and PUT. */
export const taskInputSchema = z.object(
  {
    title,
    status: status.default('todo'),
    due_date: dueDate.default(null),
    tags: tags.default([]),
    owner_id: z.int('must be an integer').nullable().default(null),
  },
  'must be a JSON object',
);
export type TaskInput = z.infer<typeof taskInputSchema>;

/** Body of PATCH: an absent field means "unchanged". */
export const taskPatchSchema = z.object(
  {
    title: title.optional(),
    status: status.optional(),
    due_date: dueDate.optional(),
    tags: tags.optional(),
  },
  'must be a JSON object',
);
export type TaskPatch = z.infer<typeof taskPatchSchema>;

const wholeNumber = z
  .string('must be given once')
  .regex(/^\d+$/, 'must be a non-negative integer')
  .transform(Number);

/** Query string of GET /tasks. Invalid values are errors; limits above 100 are clamped. */
export const listQuerySchema = z.object({
  status: status.optional(),
  tag: z.string().min(1, 'must not be empty').optional(),
  limit: wholeNumber
    .refine((n) => n >= 1, 'must be at least 1')
    .transform((n) => Math.min(n, 100))
    .default(20),
  offset: wholeNumber.default(0),
  sort: z
    .enum(['created_at', '-created_at', 'due_date', '-due_date'], 'unsupported sort')
    .default('-created_at'),
});
export type ListQuery = z.infer<typeof listQuerySchema>;

/** Flattens zod issues into `{ "tags.1": "message" }`. */
export function zodFields(error: z.ZodError): Record<string, string> {
  const fields: Record<string, string> = {};
  for (const issue of error.issues) {
    fields[issue.path.length > 0 ? issue.path.join('.') : 'body'] ??= issue.message;
  }
  return fields;
}

export interface Task extends TaskInput {
  id: number;
  created_at: string;
}

export interface Page<T> {
  items: T[];
  total: number;
  limit: number;
  offset: number;
}

export class TaskStore {
  readonly #tasks = new Map<number, Task>();
  #lastId = 0;
  readonly #clock: () => Date;

  constructor(clock: () => Date = () => new Date()) {
    this.#clock = clock;
  }

  create(input: TaskInput): Task {
    this.#lastId += 1;
    const created_at = this.#clock()
      .toISOString()
      .replace(/\.\d+Z$/, 'Z');
    const task: Task = { id: this.#lastId, ...input, created_at };
    this.#tasks.set(task.id, task);
    return task;
  }

  get(id: number): Task | undefined {
    return this.#tasks.get(id);
  }

  /** Full replacement; keeps `id` and `created_at`. */
  replace(id: number, input: TaskInput): Task | undefined {
    const current = this.#tasks.get(id);
    if (!current) return undefined;
    const task: Task = { ...input, id, created_at: current.created_at };
    this.#tasks.set(id, task);
    return task;
  }

  /** Applies only the fields present in `patch`. */
  patch(id: number, patch: TaskPatch): Task | undefined {
    const current = this.#tasks.get(id);
    if (!current) return undefined;
    const task: Task = { ...current };
    if (patch.title !== undefined) task.title = patch.title;
    if (patch.status !== undefined) task.status = patch.status;
    if (patch.due_date !== undefined) task.due_date = patch.due_date;
    if (patch.tags !== undefined) task.tags = patch.tags;
    this.#tasks.set(id, task);
    return task;
  }

  remove(id: number): boolean {
    return this.#tasks.delete(id);
  }

  list(query: ListQuery): Page<Task> {
    const matches = [...this.#tasks.values()].filter(
      (task) =>
        (query.status === undefined || task.status === query.status) &&
        (query.tag === undefined || task.tags.includes(query.tag)),
    );
    const descending = query.sort.startsWith('-');
    const key = query.sort.endsWith('due_date') ? 'due_date' : 'created_at';
    matches.sort((a, b) => {
      const left = a[key];
      const right = b[key];
      if (left === right) return a.id - b.id;
      if (left === null) return 1;
      if (right === null) return -1;
      return (left < right ? -1 : 1) * (descending ? -1 : 1);
    });
    return {
      items: matches.slice(query.offset, query.offset + query.limit),
      total: matches.length,
      limit: query.limit,
      offset: query.offset,
    };
  }
}
