import { z } from 'zod';

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

const tag = z
  .string('must be a string')
  .min(1, 'must be 1-30 characters')
  .max(30, 'must be 1-30 characters')
  .refine((value) => value === value.toLowerCase(), 'must be lowercase');

const schema = z.object(
  {
    title: z
      .string('must be a string')
      .trim()
      .min(1, 'must be 1-200 characters')
      .max(200, 'must be 1-200 characters'),
    status: z
      .enum(['todo', 'in_progress', 'done'], 'must be todo, in_progress or done')
      .default('todo'),
    due_date: z.iso.date('must be a date like 2030-01-31').nullable().default(null),
    tags: z
      .array(tag, 'must be an array')
      .max(10, 'at most 10 tags')
      .refine((list) => new Set(list).size === list.length, 'must not contain duplicates')
      .default([]),
    owner_id: z.int('must be an integer').nullable().default(null),
  },
  'must be a JSON object',
);

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
 * Hint: build a zod schema once at module level and turn `error.issues` into `fields`.
 */
export function validateTaskInput(body: unknown): ValidationResult {
  const result = schema.safeParse(body);
  if (result.success) return { ok: true, value: result.data };

  const fields: Record<string, string> = {};
  for (const issue of result.error.issues) {
    const path = issue.path.length > 0 ? issue.path.join('.') : 'body';
    fields[path] ??= issue.message;
  }
  return { ok: false, fields };
}
