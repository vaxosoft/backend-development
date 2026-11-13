import type { FastifyInstance } from 'fastify';
import type { FileStore } from './store.ts';

export function registerFileRoutes(_app: FastifyInstance, _store: FileStore): void {
  // TODO: POST /files        -> multipart field "file", stream into store.save, 201 + record
  // TODO: GET /files/:id     -> stream back with Content-Length, Content-Type, Content-Disposition,
  //                             ETag; If-None-Match match -> 304 without a body
  // TODO: DELETE /files/:id  -> 204 (404 for unknown ids)
}
