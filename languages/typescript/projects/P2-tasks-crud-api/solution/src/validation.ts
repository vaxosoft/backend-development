import { z } from 'zod';
import {
  type ListQuery,
  type Result,
  SORTS,
  STATUSES,
  type TaskInput,
  type TaskPatch,
} from './types.ts';

const TITLE_MESSAGE = 'must be 1-200 characters';

function isCalendarDate(value: string): boolean {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return false;
  const parsed = new Date(`${value}T00:00:00Z`);
  return !Number.isNaN(parsed.getTime()) && parsed.toISOString().startsWith(value);
}

const title = z
  .string({ error: TITLE_MESSAGE })
  .trim()
  .min(1, TITLE_MESSAGE)
  .max(200, TITLE_MESSAGE);
const status = z.enum(STATUSES, { error: `must be one of ${STATUSES.join(', ')}` });
const dueDate = z
  .string({ error: 'must be a YYYY-MM-DD date or null' })
  .refine(isCalendarDate, 'must be a valid YYYY-MM-DD date')
  .nullable();
const tag = z
  .string({ error: 'tags must be strings' })
  .min(1, 'each tag must be 1-30 characters')
  .max(30, 'each tag must be 1-30 characters')
  .refine((value) => value === value.toLowerCase(), 'tags must be lowercase');
const tags = z
  .array(tag, { error: 'must be an array of tags' })
  .max(10, 'at most 10 tags')
  .refine((values) => new Set(values).size === values.length, 'tags must be unique');
const ownerId = z.int({ error: 'must be an integer or null' }).nullable();

const taskInput = z.object(
  {
    title,
    status: status.default('todo'),
    due_date: dueDate.default(null),
    tags: tags.default([]),
    owner_id: ownerId.default(null),
  },
  { error: 'must be a JSON object' },
);

const taskPatch = z.object(
  {
    title: title.optional(),
    status: status.optional(),
    due_date: dueDate.optional(),
    tags: tags.optional(),
  },
  { error: 'must be a JSON object' },
);

const intParam = (message: string) =>
  z
    .string({ error: message })
    .regex(/^\d+$/, message)
    .transform(Number)
    .refine(Number.isSafeInteger, message);

const listQuery = z.object({
  status: status.optional(),
  tag: z.string().min(1).max(30).optional(),
  limit: intParam('must be an integer >= 1')
    .refine((n) => n >= 1, 'must be an integer >= 1')
    .transform((n) => Math.min(n, 100))
    .default(20),
  offset: intParam('must be an integer >= 0').default(0),
  sort: z.enum(SORTS, { error: `must be one of ${SORTS.join(', ')}` }).default('-created_at'),
});

/** Keeps the first message per top-level field: that is what the envelope's `fields` shows. */
function toResult<T>(parsed: z.ZodSafeParseResult<T>): Result<T> {
  if (parsed.success) return { ok: true, value: parsed.data };
  const fields: Record<string, string> = {};
  for (const issue of parsed.error.issues) {
    fields[String(issue.path[0] ?? 'body')] ??= issue.message;
  }
  return { ok: false, fields };
}

export function parseTaskInput(body: unknown): Result<TaskInput> {
  return toResult(taskInput.safeParse(body));
}

export function parseTaskPatch(body: unknown): Result<TaskPatch> {
  return toResult(taskPatch.safeParse(body));
}

export function parseListQuery(query: unknown): Result<ListQuery> {
  return toResult(listQuery.safeParse(query));
}
