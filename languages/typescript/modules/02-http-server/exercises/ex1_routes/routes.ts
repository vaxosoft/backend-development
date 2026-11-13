import type { FastifyInstance } from 'fastify';

/**
 * Builds the app (without listening). Routes:
 * - GET /healthz           -> 200 {"status":"ok"}
 * - GET /hello             -> 200 text/plain "Hello, world!"
 * - GET /hello/:name       -> 200 text/plain "Hello, <name>!"; with ?shout=true the whole text is upper-cased
 * - GET /items/:id         -> 200 {"id": <number>}; a non-integer id -> 400 {"error": "id must be an integer"}
 * - POST /echo             -> 201 {"received": <parsed JSON body>}
 * - any other route        -> 404 {"error": "not found"}
 */
export function buildApp(): FastifyInstance {
  throw new Error('TODO');
}
