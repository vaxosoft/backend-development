// Shared by the REST examples: error envelope, zod schemas and list-query handling.
// Not runnable on its own.
import type { FastifyInstance } from 'fastify';
import { z } from 'zod';

// ---- error envelope -------------------------------------------------------------------------

export type ErrorCode =
  | 'bad_request'
  | 'not_found'
  | 'conflict'
  | 'validation_failed'
  | 'payload_too_large'
  | 'internal';

export interface ErrorBody {
  error: { code: ErrorCode; message: string; fields?: Record<string, string> };
}

/** An error that already knows which HTTP response it should become. */
export class HttpError extends Error {
  readonly statusCode: number;
  readonly code: ErrorCode;
  readonly fields: Record<string, string> | undefined;

  constructor(
    statusCode: number,
    code: ErrorCode,
    message: string,
    fields?: Record<string, string>,
  ) {
    super(message);
    this.name = 'HttpError';
    this.statusCode = statusCode;
    this.code = code;
    this.fields = fields;
  }
}

export const notFound = (what: string): HttpError =>
  new HttpError(404, 'not_found', `${what} not found`);

/** Installs the JSON 404 handler and the one error handler that builds every error response. */
export function registerErrorHandling(app: FastifyInstance): void {
  app.setNotFoundHandler(async (req, reply) =>
    reply.code(404).send(body('not_found', `route ${req.method} ${req.url} not found`)),
  );

  app.setErrorHandler(async (error, req, reply) => {
    if (error instanceof HttpError) {
      return reply.code(error.statusCode).send(body(error.code, error.message, error.fields));
    }
    // Errors raised by Fastify itself (malformed JSON, body too large, ...) carry a 4xx statusCode.
    const status = statusOf(error);
    if (status === 413) return reply.code(413).send(body('payload_too_large', 'payload too large'));
    if (status !== undefined && status >= 400 && status < 500) {
      return reply.code(status).send(body('bad_request', 'malformed request'));
    }
    req.log.error(error); // details go to the log, never to the client
    return reply.code(500).send(body('internal', 'internal server error'));
  });
}

/** Fastify errors are `unknown` in handlers: narrow before reading `statusCode`. */
function statusOf(error: unknown): number | undefined {
  if (typeof error === 'object' && error !== null && 'statusCode' in error) {
    return typeof error.statusCode === 'number' ? error.statusCode : undefined;
  }
  return undefined;
}

function body(code: ErrorCode, message: string, fields?: Record<string, string>): ErrorBody {
  return { error: fields ? { code, message, fields } : { code, message } };
}

// ---- validation -----------------------------------------------------------------------------

/** Flattens zod issues into `{ "tags.1": "message" }`, keeping the first message per path. */
export function zodFields(error: z.ZodError): Record<string, string> {
  const fields: Record<string, string> = {};
  for (const issue of error.issues) {
    const path = issue.path.length > 0 ? issue.path.join('.') : 'body';
    fields[path] ??= issue.message;
  }
  return fields;
}

const title = z
  .string('must be a string')
  .trim()
  .min(1, 'must be 1-200 characters')
  .max(200, 'must be 1-200 characters');
const status = z.enum(['todo', 'in_progress', 'done'], 'must be one of todo, in_progress, done');
const dueDate = z.iso.date('must be a date like 2030-01-31').nullable();
const tag = z
  .string('must be a string')
  .min(1, 'must be 1-30 characters')
  .max(30, 'must be 1-30 characters')
  .refine((value) => value === value.toLowerCase(), 'must be lowercase');
const tags = z
  .array(tag, 'must be an array')
  .max(10, 'at most 10 tags')
  .refine((list) => new Set(list).size === list.length, 'must not contain duplicates');
const ownerId = z.int('must be an integer').nullable();

/** Body of POST and PUT: absent optional fields take their defaults. */
export const taskInputSchema = z.object(
  {
    title,
    status: status.default('todo'),
    due_date: dueDate.default(null),
    tags: tags.default([]),
    owner_id: ownerId.default(null),
  },
  'must be a JSON object',
);
export type TaskInput = z.infer<typeof taskInputSchema>;

/** Body of PATCH: only these fields, and an absent field means "unchanged". */
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

export interface Task extends TaskInput {
  id: number;
  created_at: string;
}

/** Validates untrusted data and throws the 422 envelope error when it is invalid. */
export function parseOr422<T>(schema: z.ZodType<T>, data: unknown, what: string): T {
  const result = schema.safeParse(data);
  if (!result.success)
    throw new HttpError(422, 'validation_failed', `invalid ${what}`, zodFields(result.error));
  return result.data;
}

// ---- list queries: filtering, sorting, pagination -------------------------------------------

const wholeNumber = z
  .string('must be given once')
  .regex(/^\d+$/, 'must be a non-negative integer')
  .transform(Number);

export const listQuerySchema = z.object({
  status: status.optional(),
  tag: z.string().min(1, 'must not be empty').optional(),
  limit: wholeNumber
    .refine((n) => n >= 1, 'must be at least 1')
    .transform((n) => Math.min(n, 100)) // larger values are clamped, not rejected
    .default(20),
  offset: wholeNumber.default(0),
  sort: z
    .enum(['created_at', '-created_at', 'due_date', '-due_date'], 'unsupported sort')
    .default('-created_at'),
});
export type ListQuery = z.infer<typeof listQuerySchema>;

export interface Page<T> {
  items: T[];
  total: number;
  limit: number;
  offset: number;
}

/** Parses query-string values (strings) into a typed `ListQuery`; invalid values become 400. */
export function parseListQuery(query: unknown): ListQuery {
  const result = listQuerySchema.safeParse(query);
  if (!result.success) {
    const [field, message] = Object.entries(zodFields(result.error))[0] ?? ['query', 'invalid'];
    throw new HttpError(400, 'bad_request', `invalid query parameter ${field}: ${message}`);
  }
  return result.data;
}

/** Filters, sorts and slices `tasks`. `total` counts matches before slicing. */
export function listTasks(tasks: readonly Task[], query: ListQuery): Page<Task> {
  const matches = tasks.filter(
    (task) =>
      (query.status === undefined || task.status === query.status) &&
      (query.tag === undefined || task.tags.includes(query.tag)),
  );

  const descending = query.sort.startsWith('-');
  const key = query.sort.endsWith('due_date') ? 'due_date' : 'created_at';
  matches.sort((a, b) => {
    const left = a[key];
    const right = b[key];
    if (left === right) return a.id - b.id; // stable order for ties
    if (left === null) return 1; // tasks without a due date always come last
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
