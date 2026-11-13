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

/** Applies `fn` to the value of a successful result; failures pass through unchanged. */
export function mapResult<T, U, E>(_result: Result<T, E>, _fn: (value: T) => U): Result<U, E> {
  throw new Error('TODO');
}

/** Parses JSON text. On failure returns a `ParseError` whose `cause` is the original error. */
export function tryParseJson(_text: string): Result<unknown, ParseError> {
  throw new Error('TODO');
}

/**
 * Validates untrusted input into `{ title, status }`:
 * - the input must be a non-null, non-array object (otherwise `fields.body`)
 * - `title` must be a string of 1-200 characters after trimming (the trimmed value is returned)
 * - `status` is optional (default `todo`) and must be a `Status`
 * Report every problem at once, not only the first one.
 */
export function parseDraft(
  _input: unknown,
): Result<{ title: string; status: Status }, ValidationError> {
  throw new Error('TODO');
}

/** Returns the value of a successful result or throws the error itself. */
export function unwrap<T, E extends Error>(_result: Result<T, E>): T {
  throw new Error('TODO');
}
