export type Result<T, E> = { ok: true; value: T } | { ok: false; error: E };

export const ok = <T>(value: T): Result<T, never> => ({ ok: true, value });
export const err = <E>(error: E): Result<never, E> => ({ ok: false, error });

export type Status = 'todo' | 'in_progress' | 'done';

export class ParseError extends Error {
  constructor(message: string, options?: ErrorOptions) {
    super(message, options);
    this.name = 'ParseError';
  }
}

export class ValidationError extends Error {
  readonly fields: Readonly<Record<string, string>>;

  constructor(fields: Record<string, string>) {
    super(`invalid input: ${Object.keys(fields).join(', ')}`);
    this.name = 'ValidationError';
    this.fields = fields;
  }
}

const STATUSES: readonly Status[] = ['todo', 'in_progress', 'done'];
const isStatus = (value: unknown): value is Status => STATUSES.some((s) => s === value);

/** Applies `fn` to the value of a successful result; failures pass through unchanged. */
export function mapResult<T, U, E>(result: Result<T, E>, fn: (value: T) => U): Result<U, E> {
  return result.ok ? ok(fn(result.value)) : result;
}

/** Parses JSON text. On failure returns a `ParseError` whose `cause` is the original error. */
export function tryParseJson(text: string): Result<unknown, ParseError> {
  try {
    return ok(JSON.parse(text));
  } catch (error) {
    return err(new ParseError('invalid JSON', { cause: error }));
  }
}

/**
 * Validates untrusted input into `{ title, status }`:
 * - the input must be a non-null, non-array object (otherwise `fields.body`)
 * - `title` must be a string of 1-200 characters after trimming (the trimmed value is returned)
 * - `status` is optional (default `todo`) and must be a `Status`
 * Report every problem at once, not only the first one.
 */
export function parseDraft(
  input: unknown,
): Result<{ title: string; status: Status }, ValidationError> {
  if (typeof input !== 'object' || input === null || Array.isArray(input)) {
    return err(new ValidationError({ body: 'must be an object' }));
  }
  const record: Record<string, unknown> = { ...input };
  const fields: Record<string, string> = {};

  const title = typeof record.title === 'string' ? record.title.trim() : '';
  if (title.length < 1 || title.length > 200) fields.title = 'must be 1-200 characters';

  const status = record.status ?? 'todo';
  if (!isStatus(status)) fields.status = 'must be one of todo, in_progress, done';

  if (!isStatus(status) || Object.keys(fields).length > 0) return err(new ValidationError(fields));
  return ok({ title, status });
}

/** Returns the value of a successful result or throws the error itself. */
export function unwrap<T, E extends Error>(result: Result<T, E>): T {
  if (!result.ok) throw result.error;
  return result.value;
}
