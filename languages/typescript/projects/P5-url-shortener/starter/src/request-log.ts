import type { FastifyInstance } from 'fastify';
import type { Logger } from 'pino';

/**
 * TODO(P5): write exactly one JSON line per finished request through `logger`
 * (an `onResponse` hook) with the fields
 * `request_id` (request.id), `method`, `path` (no query string, it may hold secrets), `status`,
 * `duration_ms` (reply.elapsedTime) and `user_id` (request.userId) only when the user is known.
 * Never log bodies or the Authorization header.
 */
export function registerRequestLogging(_app: FastifyInstance, _logger: Logger): void {}
