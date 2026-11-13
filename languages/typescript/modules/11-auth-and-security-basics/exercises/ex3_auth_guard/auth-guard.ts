import type { FastifyInstance } from 'fastify';

export type Role = 'admin' | 'member';

export interface Claims {
  sub: number;
  role: Role;
}

export type Take = { allowed: true } | { allowed: false; retryAfterSeconds: number };

export interface Deps {
  /** Resolves to the claims of a valid token and to `null` for any invalid one. May reject: treat that as invalid. */
  verifyToken(token: string): Promise<Claims | null>;
  /** One bucket per key; `take` consumes one unit. */
  limiter: { take(key: string): Take };
}

/**
 * Builds the secured Task Tracker API. Every error body is `{"error":{"code","message"}}`.
 *
 * Authentication (all routes below except `/healthz`):
 * - credentials come from `Authorization: Bearer <token>` (scheme matched case-insensitively);
 *   a missing header, another scheme (`Basic ...`), an empty token, an invalid token or a
 *   `verifyToken` failure -> 401 `unauthorized` with a `WWW-Authenticate: Bearer` header.
 *
 * Routes:
 * - `GET /healthz` -> 200 `{"status":"ok"}`, no authentication.
 * - `GET /me` -> 200 `{"id": <sub>, "role": <role>}`.
 * - `GET /admin/stats` -> 200 `{"users": 2}` for admins; members get 403 `forbidden`.
 * - `POST /tasks` -> 201 `{"id":1,"title":..., "owner_id": <sub>}`. Rate limited per user with
 *   `limiter.take("user:<sub>")`: when not allowed, 429 `rate_limited` with `Retry-After: <seconds>`.
 *   The limiter is consulted only for authenticated users, and before the body is validated.
 *   The body needs a non-empty `title` (after trimming, at most 200 characters), else 422 `validation_failed`.
 *
 * Every response (including errors) carries `X-Content-Type-Options: nosniff`;
 * responses of authenticated routes also carry `Cache-Control: no-store`.
 */
export function buildApp(_deps: Deps): FastifyInstance {
  throw new Error('TODO');
}
