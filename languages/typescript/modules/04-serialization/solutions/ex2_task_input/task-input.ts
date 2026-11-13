import { z } from 'zod';

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

const Title = z
  .string()
  .trim()
  .min(1, 'must be 1-200 characters')
  .max(200, 'must be 1-200 characters');
const Status = z.enum(['todo', 'in_progress', 'done']);
const DueDate = z.iso.date('must be a YYYY-MM-DD date');
const Tags = z
  .array(
    z
      .string()
      .min(1)
      .max(30)
      .refine((t) => t === t.toLowerCase(), 'must be lowercase'),
  )
  .max(10)
  .refine((tags) => new Set(tags).size === tags.length, 'must be unique');

const CreateSchema = z.strictObject({
  title: Title,
  status: Status.default('todo'),
  due_date: DueDate.nullable().default(null),
  tags: Tags.default([]),
  owner_id: z.number().int().nullable().default(null),
});

const PatchSchema = z
  .strictObject({ title: Title, status: Status, due_date: DueDate.nullable(), tags: Tags })
  .partial();

function toFields(error: z.ZodError): Record<string, string> {
  const fields: Record<string, string> = {};
  for (const issue of error.issues) {
    if (issue.code === 'unrecognized_keys') {
      for (const key of issue.keys) fields[key] = 'unknown field';
    } else {
      fields[String(issue.path[0] ?? '_')] ??= issue.message;
    }
  }
  return fields;
}

function parseWith<T>(schema: z.ZodType<T>, body: unknown): Parsed<T> {
  const result = schema.safeParse(body);
  return result.success
    ? { ok: true, value: result.data }
    : { ok: false, fields: toFields(result.error) };
}

/**
 * Validates a decoded JSON body for POST /tasks.
 * title: trimmed, 1-200 chars. status: todo|in_progress|done, default todo.
 * due_date: real YYYY-MM-DD date or null (default null). tags: 0-10 unique lowercase strings of 1-30 chars (default []).
 * owner_id: integer or null (default null). Unknown keys are rejected.
 */
export function parseCreateTask(body: unknown): Parsed<CreateTaskInput> {
  return parseWith(CreateSchema, body);
}

/**
 * Validates a decoded JSON body for PATCH /tasks/{id}: only title, status, due_date and tags are allowed.
 * An absent key stays absent in the result; `due_date: null` stays null (it clears the date).
 */
export function parsePatchTask(body: unknown): Parsed<PatchTaskInput> {
  return parseWith(PatchSchema, body);
}
