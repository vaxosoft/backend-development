export interface TaskInput {
  title: string;
  status: 'todo' | 'in_progress' | 'done';
  due_date: string | null;
  tags: string[];
  owner_id: number | null;
}

export type ValidationResult =
  | { ok: true; value: TaskInput }
  | { ok: false; fields: Record<string, string> };

/**
 * Validates an untrusted request body for creating a task. Rules:
 * - the body must be a JSON object (otherwise report the problem under the key `body`)
 * - `title`: required string, trimmed (the trimmed value is returned), 1-200 characters
 * - `status`: `todo` (default), `in_progress` or `done`
 * - `due_date`: `YYYY-MM-DD` of a real calendar date, or null (default null)
 * - `tags`: array (default []) of at most 10 tags; each 1-30 characters, lowercase; no duplicates
 * - `owner_id`: integer or null (default null)
 * - unknown fields are dropped
 * Report ALL problems at once as `fields`, keyed by path: `title`, `tags` (for the list as a whole,
 * e.g. too long or duplicates) or `tags.1` (for one element). One message per key.
 * Hint: `import { z } from 'zod'`, build a schema once at module level and turn `error.issues` into `fields`.
 */
export function validateTaskInput(_body: unknown): ValidationResult {
  throw new Error('TODO');
}
