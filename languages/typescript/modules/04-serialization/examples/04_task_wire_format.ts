// Task Tracker wire format: snake_case JSON in and out, validation errors in the shared envelope.
import { z } from 'zod';

const TITLE_MESSAGE = 'must be 1-200 characters';

const Status = z.enum(['todo', 'in_progress', 'done']);
const Title = z.string().trim().min(1, TITLE_MESSAGE).max(200, TITLE_MESSAGE);
const DueDate = z.iso.date('must be a YYYY-MM-DD date');
const Tags = z
  .array(
    z
      .string()
      .min(1, 'tags must be 1-30 characters')
      .max(30, 'tags must be 1-30 characters')
      .refine((t) => t === t.toLowerCase(), 'tags must be lowercase'),
  )
  .max(10, 'at most 10 tags')
  .refine((tags) => new Set(tags).size === tags.length, 'tags must be unique');

const CreateTask = z.strictObject({
  title: Title,
  status: Status.default('todo'),
  due_date: DueDate.nullable().default(null),
  tags: Tags.default([]),
  owner_id: z.number().int().nullable().default(null),
});

// PATCH: an absent key means "leave unchanged", an explicit null on due_date means "clear it".
const PatchTask = z
  .strictObject({
    title: Title,
    status: Status,
    due_date: DueDate.nullable(),
    tags: Tags,
  })
  .partial();

type CreateTask = z.infer<typeof CreateTask>;

interface Task extends CreateTask {
  id: number;
  created_at: string;
}

/** Collapses zod issues into the `fields` map of the error envelope (first message per field). */
function toEnvelope(error: z.ZodError): string {
  const fields: Record<string, string> = {};
  for (const issue of error.issues) {
    if (issue.code === 'unrecognized_keys') {
      for (const key of issue.keys) fields[key] = 'unknown field';
    } else {
      fields[issue.path.join('.')] ??= issue.message;
    }
  }
  return JSON.stringify({
    error: { code: 'validation_failed', message: 'invalid task', fields },
  });
}

function rfc3339(date: Date): string {
  return date.toISOString().replace('.000Z', 'Z');
}

function create(body: unknown, id: number, now: Date): Task | string {
  const parsed = CreateTask.safeParse(body);
  if (!parsed.success) return toEnvelope(parsed.error);
  return { id, ...parsed.data, created_at: rfc3339(now) };
}

const now = new Date('2030-01-01T09:00:00Z');

const created = create({ title: '  Write docs ', tags: ['docs', 'p1'] }, 1, now);
console.log(JSON.stringify(created));

console.log(create({ title: '', due_date: '2030-02-30', tags: ['Docs', 'a', 'a'], x: 1 }, 2, now));

for (const body of [{ title: 'New' }, { due_date: null }, { status: 'x' }]) {
  const patch = PatchTask.safeParse(body);
  if (!patch.success) {
    console.log(JSON.stringify(body), '->', toEnvelope(patch.error));
  } else {
    console.log(
      JSON.stringify(body),
      '-> keys:',
      Object.keys(patch.data),
      'clears due:',
      patch.data.due_date === null,
    );
  }
}

// Malformed JSON is a different error class (400 bad_request) from valid JSON with bad data (422).
try {
  JSON.parse('{"title":');
} catch (err: unknown) {
  if (err instanceof SyntaxError) console.log('400 bad_request');
}
