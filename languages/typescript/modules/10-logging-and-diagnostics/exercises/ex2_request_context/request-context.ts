import type { Logger } from 'pino';

export interface LogSink {
  write(line: string): void;
}

/**
 * Returns the client-supplied request id when it is 1-64 characters of `[A-Za-z0-9_.-]`;
 * otherwise (missing, empty, too long, spaces, control characters...) returns `generate()`.
 */
export function resolveRequestId(_header: string | undefined, _generate?: () => string): string {
  throw new Error('TODO');
}

/** Runs `fn` so that `currentRequestId()` returns `requestId` anywhere below it, even after `await`. */
export function runWithRequestContext<T>(_requestId: string, _fn: () => T): T {
  throw new Error('TODO');
}

/** The id of the request being handled, or `undefined` outside of `runWithRequestContext`. */
export function currentRequestId(): string | undefined {
  throw new Error('TODO');
}

/**
 * A pino logger writing to `sink` that adds `request_id` to every line logged inside a request
 * context, and omits the field outside of one. Callers never pass the id explicitly.
 */
export function createContextLogger(_sink: LogSink): Logger {
  throw new Error('TODO');
}
