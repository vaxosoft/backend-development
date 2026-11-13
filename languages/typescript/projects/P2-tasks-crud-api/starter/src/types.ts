export const STATUSES = ['todo', 'in_progress', 'done'] as const;
export type Status = (typeof STATUSES)[number];

export const SORTS = ['created_at', '-created_at', 'due_date', '-due_date'] as const;
export type Sort = (typeof SORTS)[number];

/** A task as stored and as sent on the wire (snake_case JSON). */
export interface Task {
  id: number;
  title: string;
  status: Status;
  due_date: string | null;
  tags: string[];
  owner_id: number | null;
  created_at: string;
}

/** A validated body for POST and PUT: every writable field is present. */
export interface TaskInput {
  title: string;
  status: Status;
  due_date: string | null;
  tags: string[];
  owner_id: number | null;
}

/** A validated body for PATCH: absent keys mean "leave unchanged". */
export interface TaskPatch {
  title?: string;
  status?: Status;
  due_date?: string | null;
  tags?: string[];
}

/** Validated and normalised query parameters of GET /tasks. */
export interface ListQuery {
  status?: Status;
  tag?: string;
  limit: number;
  offset: number;
  sort: Sort;
}

export interface Page<T> {
  items: T[];
  total: number;
}

export type Result<T> = { ok: true; value: T } | { ok: false; fields: Record<string, string> };
