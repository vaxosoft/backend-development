// The Task Tracker domain: types, generics, composition, validation, and an in-memory store.
import {
  err,
  isTaskStatus,
  type NewTask,
  NotFoundError,
  ok,
  type Result,
  TASK_STATUSES,
  type Task,
  ValidationError,
} from './_shared.ts';

/** Anything with a numeric id can live in a `Repository`. */
interface Entity {
  readonly id: number;
}

interface Repository<T extends Entity, TNew> {
  create(input: TNew): T;
  get(id: number): T;
  list(predicate?: (item: T) => boolean): T[];
  delete(id: number): void;
}

// Composition: the store receives its collaborators (clock) instead of inheriting them.
interface Clock {
  now(): Date;
}

class InMemoryTasks implements Repository<Task, NewTask> {
  readonly #items = new Map<number, Task>();
  #nextId = 1;
  readonly #clock: Clock;

  constructor(clock: Clock) {
    this.#clock = clock;
  }

  create(input: NewTask): Task {
    const task: Task = {
      id: this.#nextId++,
      title: input.title,
      status: input.status ?? 'todo',
      due_date: input.due_date ?? null,
      tags: [...(input.tags ?? [])],
      owner_id: input.owner_id ?? null,
      created_at: this.#clock.now().toISOString(),
    };
    this.#items.set(task.id, task);
    return task;
  }

  get(id: number): Task {
    const task = this.#items.get(id);
    if (task === undefined) throw new NotFoundError('task', id);
    return task;
  }

  list(predicate: (item: Task) => boolean = () => true): Task[] {
    return [...this.#items.values()].filter(predicate);
  }

  delete(id: number): void {
    if (!this.#items.delete(id)) throw new NotFoundError('task', id);
  }
}

// Untrusted input is `unknown`; validation produces a typed value or a list of problems.
function validateNewTask(input: unknown): Result<NewTask, ValidationError> {
  if (typeof input !== 'object' || input === null) {
    return err(new ValidationError({ body: 'must be an object' }));
  }
  const record = input as Record<string, unknown>;
  const fields: Record<string, string> = {};

  const title = typeof record.title === 'string' ? record.title.trim() : '';
  if (title.length < 1 || title.length > 200) fields.title = 'must be 1-200 characters';

  const status = record.status ?? 'todo';
  const tags = record.tags ?? [];
  if (!Array.isArray(tags) || !tags.every((tag) => typeof tag === 'string')) {
    fields.tags = 'must be an array of strings';
  }

  if (!isTaskStatus(status)) fields.status = `must be one of ${TASK_STATUSES.join(', ')}`;

  if (!isTaskStatus(status) || !Array.isArray(tags) || Object.keys(fields).length > 0) {
    return err(new ValidationError(fields));
  }
  return ok({ title, status, tags: tags.map(String) });
}

// A generic helper: group any list by a computed key.
function groupBy<T, K extends string>(items: readonly T[], key: (item: T) => K): Map<K, T[]> {
  const groups = new Map<K, T[]>();
  for (const item of items) {
    const bucket = groups.get(key(item));
    if (bucket) bucket.push(item);
    else groups.set(key(item), [item]);
  }
  return groups;
}

const clock: Clock = { now: () => new Date('2030-01-01T09:00:00Z') }; // fixed: deterministic output
const tasks = new InMemoryTasks(clock);

const inputs: unknown[] = [
  { title: '  Write docs  ', tags: ['docs'] },
  { title: 'Ship it', status: 'in_progress' },
  { title: '', status: 'nope' },
  'not an object',
];
for (const input of inputs) {
  const result = validateNewTask(input);
  if (result.ok) {
    console.log('created', JSON.stringify(tasks.create(result.value)));
  } else {
    console.log('rejected', JSON.stringify(result.error.fields));
  }
}

const byStatus = groupBy(tasks.list(), (task) => task.status);
for (const [status, items] of byStatus) {
  console.log(
    status,
    items.map((task) => task.title),
  );
}

try {
  tasks.get(42);
} catch (error) {
  console.log(error instanceof NotFoundError ? error.message : 'unexpected');
}
