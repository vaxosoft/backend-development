import { AsyncLocalStorage } from 'node:async_hooks';
import { randomUUID } from 'node:crypto';
import pino, { type Logger } from 'pino';

export interface LogSink {
  write(line: string): void;
}

const storage = new AsyncLocalStorage<string>();
const REQUEST_ID_PATTERN = /^[\w.-]{1,64}$/;

/**
 * Returns the client-supplied request id when it is 1-64 characters of `[A-Za-z0-9_.-]`;
 * otherwise (missing, empty, too long, spaces, control characters...) returns `generate()`.
 */
export function resolveRequestId(
  header: string | undefined,
  generate: () => string = randomUUID,
): string {
  return header !== undefined && REQUEST_ID_PATTERN.test(header) ? header : generate();
}

/** Runs `fn` so that `currentRequestId()` returns `requestId` anywhere below it, even after `await`. */
export function runWithRequestContext<T>(requestId: string, fn: () => T): T {
  return storage.run(requestId, fn);
}

/** The id of the request being handled, or `undefined` outside of `runWithRequestContext`. */
export function currentRequestId(): string | undefined {
  return storage.getStore();
}

/**
 * A pino logger writing to `sink` that adds `request_id` to every line logged inside a request
 * context, and omits the field outside of one. Callers never pass the id explicitly.
 */
export function createContextLogger(sink: LogSink): Logger {
  return pino({ base: null, mixin: () => ({ request_id: currentRequestId() }) }, sink);
}
