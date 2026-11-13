export type Status = 'todo' | 'in_progress' | 'done';

export interface CreateTaskInput {
  title: string;
  status: Status;
  due_date: string | null;
  tags: string[];
  owner_id: number | null;
}

export interface PatchTaskInput {
  title?: string;
  status?: Status;
  due_date?: string | null;
  tags?: string[];
}

/** On failure `fields` maps the first path segment (or the unknown key) to a message; `_` means the body itself. */
export type Parsed<T> = { ok: true; value: T } | { ok: false; fields: Record<string, string> };

/**
 * Validates a decoded JSON body for POST /tasks.
 * title: trimmed, 1-200 chars. status: todo|in_progress|done, default todo.
 * due_date: real YYYY-MM-DD date or null (default null). tags: 0-10 unique lowercase strings of 1-30 chars (default []).
 * owner_id: integer or null (default null). Unknown keys are rejected.
 */
export function parseCreateTask(_body: unknown): Parsed<CreateTaskInput> {
  throw new Error('TODO');
}

/**
 * Validates a decoded JSON body for PATCH /tasks/{id}: only title, status, due_date and tags are allowed.
 * An absent key stays absent in the result; `due_date: null` stays null (it clears the date).
 */
export function parsePatchTask(_body: unknown): Parsed<PatchTaskInput> {
  throw new Error('TODO');
}
