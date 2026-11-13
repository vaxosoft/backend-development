import type { FastifyInstance } from 'fastify';
import { TaskStore } from './task-model.ts';

/**
 * Builds the Tasks REST API on top of `store` (no listening). Requirements:
 *
 * Routes
 * - POST   /tasks      validate with `taskInputSchema`; 201 + `Location: /tasks/<id>` + the task
 * - GET    /tasks      validate the query with `listQuerySchema`; 200 with the page envelope
 * - GET    /tasks/:id  200 or 404 (ids that are not whole numbers are also 404)
 * - PUT    /tasks/:id  full replace (`taskInputSchema`); 200 or 404
 * - PATCH  /tasks/:id  partial update (`taskPatchSchema`); 200 or 404
 * - DELETE /tasks/:id  204 or 404
 *
 * Error envelope (always `content-type: application/json`):
 *   {"error":{"code":"...","message":"...","fields":{...}}}
 * - invalid body                 -> 422 `validation_failed` with `fields` (use `zodFields`)
 * - invalid query parameter      -> 400 `bad_request` (no `fields`)
 * - malformed JSON               -> 400 `bad_request`
 * - body above 1 KiB             -> 413 `payload_too_large` (set Fastify's `bodyLimit`)
 * - unknown task or route        -> 404 `not_found`
 * - any unexpected error         -> 500 `internal` with the generic message "internal server error"
 *   (never leak the original message)
 *
 * Idempotency: a POST with an `Idempotency-Key` header is executed once. Repeating it with the same
 * body returns 201 with the same task, the same Location and the header `idempotent-replayed: true`
 * without creating another task. Reusing the key with a different body -> 409 `conflict`.
 */
export function buildApi(_store: TaskStore = new TaskStore()): FastifyInstance {
  throw new Error('TODO');
}
