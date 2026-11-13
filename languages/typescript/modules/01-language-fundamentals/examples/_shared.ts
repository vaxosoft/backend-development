// Shared by examples 03 and 04. Not runnable on its own.

export type Result<T, E> = { ok: true; value: T } | { ok: false; error: E };

export const ok = <T>(value: T): Result<T, never> => ({ ok: true, value });
export const err = <E>(error: E): Result<never, E> => ({ ok: false, error });

export const TASK_STATUSES = ['todo', 'in_progress', 'done'] as const;
export type TaskStatus = (typeof TASK_STATUSES)[number];

export const isTaskStatus = (value: unknown): value is TaskStatus =>
  TASK_STATUSES.some((status) => status === value);

export interface Task {
  readonly id: number;
  title: string;
  status: TaskStatus;
  due_date: string | null;
  tags: string[];
  owner_id: number | null;
  readonly created_at: string;
}

export type NewTask = Pick<Task, 'title'> & Partial<Omit<Task, 'id' | 'title' | 'created_at'>>;

export class ValidationError extends Error {
  readonly fields: Readonly<Record<string, string>>;

  constructor(fields: Record<string, string>, options?: ErrorOptions) {
    super(`invalid task: ${Object.keys(fields).join(', ')}`, options);
    this.name = 'ValidationError';
    this.fields = fields;
  }
}

export class NotFoundError extends Error {
  constructor(resource: string, id: number, options?: ErrorOptions) {
    super(`${resource} ${id} not found`, options);
    this.name = 'NotFoundError';
  }
}
