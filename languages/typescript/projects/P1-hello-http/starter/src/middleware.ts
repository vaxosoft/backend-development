import type { FastifyInstance } from 'fastify';

/** Echo the request's X-Request-ID on every response (generated when absent). */
export function registerRequestId(_app: FastifyInstance): void {
  // TODO: add an onRequest hook that sets the `x-request-id` response header to `req.id`.
}

/** Write one JSON line per finished request to `sink`. */
export function registerRequestLogger(_app: FastifyInstance, _sink: (line: string) => void): void {
  // TODO: add an onResponse hook. Line fields: request_id, method, path (no query),
  // status and duration_ms (number).
}

/** Turn thrown errors and unknown routes into the shared JSON error envelope. */
export function registerRecovery(
  _app: FastifyInstance,
  _errorSink: (error: unknown) => void,
): void {
  // TODO: setErrorHandler (500 `internal`, never leak the message; report the real error
  // to `errorSink`; keep 4xx status codes) and setNotFoundHandler (404 `not_found`).
}
