# P5 - URL Shortener

A small service with accounts: users register, log in for a JWT, create short links, and anyone can follow `/{slug}`. It adds the pieces a public service needs: password hashing, token verification, per-user rate limiting and structured request logs.

**Modules exercised:** [03 REST API design](../../modules/03-rest-api-design/README.md), [05 Database access](../../modules/05-database-access/README.md), [10 Logging and diagnostics](../../modules/10-logging-and-diagnostics/README.md), [11 Auth and security basics](../../modules/11-auth-and-security-basics/README.md).

## Requirements

All JSON errors use the shared envelope `{"error":{"code","message","fields"?}}` (see [domain-and-projects](../../../../framework/domain-and-projects.md)). Already provided in the starter: envelope, validation schemas, SQLite schema, wiring.

| Endpoint                                  | Rules                                                                                                                                                                                                                                                                                             |
| ----------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `POST /auth/register` `{email, password}` | `201 {id, email}`; email must contain `@` (trimmed, lower-cased), password at least 8 characters (else `422 validation_failed` with `fields`); the password is stored as a salted hash; email already registered -> `409 conflict`; malformed JSON -> `400 bad_request`                           |
| `POST /auth/login` `{email, password}`    | `200 {token}`, an HS256 JWT with `sub` = user id that expires after 1 hour; wrong password and unknown user both -> `401 unauthorized` with the **same** body                                                                                                                                     |
| `POST /links` (auth) `{url, slug?}`       | `201 {slug, url}`; `url` absolute `http`/`https` (else 422 on `url`); `slug` optional, 3-32 characters `[A-Za-z0-9_-]` (else 422 on `slug`), generated when absent as 7 characters `[A-Za-z0-9]`; slug already taken (by anyone) -> `409 conflict`; the slug `links` is reserved -> `409`         |
| `GET /{slug}` (public)                    | `302` with `Location`; each hit is counted; unknown slug -> `404 not_found`                                                                                                                                                                                                                       |
| `GET /links` (auth)                       | the caller's own links `{"items":[{slug,url,hits,created_at}],"total","limit","offset"}`, oldest first; `limit` default 20, larger values clamped to 100; `offset` default 0; non-numeric -> `400`                                                                                                |
| `DELETE /links/{slug}` (auth)             | owner -> `204`; another user -> `403 forbidden`; unknown -> `404`                                                                                                                                                                                                                                 |
| Authentication                            | `Authorization: Bearer <jwt>`; missing, malformed, expired, wrongly signed, unsigned (`alg: none`) or non-HS256 tokens -> `401 unauthorized`. Verification pins `HS256`                                                                                                                           |
| Rate limit                                | token bucket per authenticated user on `POST /links` (`rateLimitBurst`, `rateLimitRefillPerSecond`): empty bucket -> `429 rate_limited` with `Retry-After` (whole seconds, at least 1). Every attempt costs a token, valid or not. Redirects and listing are not limited                          |
| Logging                                   | one JSON line per request through the injected log stream: `request_id`, `method`, `path` (no query string), `status`, `duration_ms`, and `user_id` when the caller is known. Passwords and tokens never appear. `X-Request-ID` is echoed (or a UUID is generated) and is the logged `request_id` |

## Configuration

`buildApp(config)` takes a complete `Config` (`starter/src/config.ts`): the clock (`now`) and the log stream are injectable, which keeps the tests free of sleeps. `src/main.ts` builds it from the environment:

| Env var                        | Default                                   |
| ------------------------------ | ----------------------------------------- |
| `ADDR`                         | `127.0.0.1:8080`                          |
| `JWT_SECRET`                   | required, at least 32 characters          |
| `DB_PATH`                      | `:memory:` (use a file name to keep data) |
| `RATE_LIMIT_BURST`             | `5`                                       |
| `RATE_LIMIT_REFILL_PER_SECOND` | `1`                                       |
| `SCRYPT_COST`                  | `16384` (tests use 1024)                  |

## Run and test

All commands run from `languages/typescript/`.

```bash
JWT_SECRET=change-me-change-me-change-me-0000 ADDR=127.0.0.1:8080 \
  node projects/P5-url-shortener/starter/src/main.ts

curl -s -X POST 127.0.0.1:8080/auth/register -H 'content-type: application/json' \
  -d '{"email":"alice@example.com","password":"correct horse"}'
TOKEN=$(curl -s -X POST 127.0.0.1:8080/auth/login -H 'content-type: application/json' \
  -d '{"email":"alice@example.com","password":"correct horse"}' | node -pe 'JSON.parse(require("fs").readFileSync(0)).token')
curl -s -X POST 127.0.0.1:8080/links -H "authorization: Bearer $TOKEN" \
  -H 'content-type: application/json' -d '{"url":"https://example.com/docs","slug":"docs"}'
curl -i 127.0.0.1:8080/docs
curl -s 127.0.0.1:8080/links -H "authorization: Bearer $TOKEN"

node --test "projects/P5-url-shortener/tests/**/*.test.ts"
```

Expected responses with a finished solution (`created_at` is the real clock time):

```text
{"id":1,"email":"alice@example.com"}
{"slug":"docs","url":"https://example.com/docs"}
HTTP/1.1 302 Found
location: https://example.com/docs
{"items":[{"slug":"docs","url":"https://example.com/docs","hits":1,"created_at":"2030-01-01T09:00:00.000Z"}],"total":1,"limit":20,"offset":0}
```

The tests use `app.inject()` (no sockets), a fresh SQLite file per test, a fake clock and a log collector. Helpers live in `tests/helpers.ts`.

## What to implement

The starter type-checks and is wired (`app.ts`, `main.ts`, config, errors, schemas, schema creation). Six `TODO(P5)` stubs are yours:

| File                         | Task                                                    | Module |
| ---------------------------- | ------------------------------------------------------- | ------ |
| `starter/src/store.ts`       | repository: prepared statements for users and links     | 05     |
| `starter/src/passwords.ts`   | `scrypt` with a random salt, constant-time verification | 11     |
| `starter/src/tokens.ts`      | sign and verify HS256 JWTs with `jose`                  | 11     |
| `starter/src/rate-limit.ts`  | token bucket per key with an injectable clock           | 11     |
| `starter/src/request-log.ts` | one structured log line per request                     | 10     |
| `starter/src/routes.ts`      | the handlers, status codes and the `requireUser` check  | 03     |

## Acceptance criteria -> tests

| Criterion                        | Test (file)                                                                                                                                                                                                                 |
| -------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| register 201, no password echoed | `register returns 201 with id and email and never echoes the password` (auth)                                                                                                                                               |
| validation 422 with fields       | `register rejects a short password and a bad email with 422 and field messages` (auth), `rejects URLs that are not absolute http(s)...`, `rejects slugs outside 3-32 characters...` (links)                                 |
| duplicate email 409              | `registering the same email twice returns 409 conflict` (auth)                                                                                                                                                              |
| malformed JSON 400               | `malformed JSON returns 400 bad_request in the error envelope` (auth)                                                                                                                                                       |
| hashed passwords                 | `passwords are stored as salted hashes, never in clear text` (auth)                                                                                                                                                         |
| JWT HS256, 1 h                   | `login returns an HS256 JWT that expires in one hour` (auth)                                                                                                                                                                |
| same 401 for both failures       | `wrong password and unknown user get the identical 401` (auth)                                                                                                                                                              |
| auth required and enforced       | `protected endpoints answer 401 without a token`, `forged, expired and wrongly signed tokens are rejected` (links)                                                                                                          |
| create link, slugs               | `generates a 7 character alphanumeric slug`, `uses a custom slug when given`, `a taken slug returns 409 conflict, also for another user`, `the slug "links" is reserved...` (links)                                         |
| redirect and 404                 | `GET /{slug} is public and answers 302 with Location`, `an unknown slug returns 404 in the error envelope` (links)                                                                                                          |
| own links, hits, paging          | `GET /links lists only my links, with hits counted per visit`, `GET /links pages with limit and offset and clamps limit to 100` (links)                                                                                     |
| delete rules                     | `the owner can delete a link...`, `another user gets 403 and the link survives`, `deleting an unknown slug returns 404` (links)                                                                                             |
| rate limit                       | `allows a burst, then answers 429 rate_limited with Retry-After`, `buckets are per user`, `tokens refill over time`, `public redirects and the listing are not rate limited` (ops)                                          |
| structured logging               | `writes one JSON line per request with the required fields`, `includes user_id once the caller is authenticated`, `echoes X-Request-ID and uses it as request_id, or generates one`, `never logs passwords or tokens` (ops) |

## Suggested steps and hints

1. `store.ts` (users), `passwords.ts`, `tokens.ts` and the two auth handlers in `routes.ts`; run `auth.test.ts`.
2. `store.ts` (links), `requireUser` and the link handlers; run `links.test.ts`.
3. `rate-limit.ts`, then `request-log.ts`; run `ops.test.ts`.

<details>
<summary>Hint: SQL without exceptions</summary>

`INSERT ... ON CONFLICT DO NOTHING` plus `result.changes === 0` tells you a unique constraint was hit without catching anything. `UPDATE links SET hits = hits + 1 WHERE slug = ? RETURNING url` counts and fetches in one atomic statement. Cast `.get()` / `.all()` results to a row interface once and map to your domain type.
</details>

<details>
<summary>Hint: scrypt</summary>

`crypto.scrypt(password, salt, 64, { N: cost }, callback)` wrapped in a `Promise`. Store `scrypt$cost$saltHex$hashHex`; on verify, `split('$')`, re-derive, compare lengths first and then use `timingSafeEqual`. For unknown users verify against a pre-computed decoy hash so both failure paths cost the same time.
</details>

<details>
<summary>Hint: jose</summary>

`new SignJWT().setProtectedHeader({ alg: 'HS256' }).setSubject(String(id)).setIssuedAt().setExpirationTime('1h').sign(new TextEncoder().encode(secret))`, and `jwtVerify(token, key, { algorithms: ['HS256'] })` in a `try`/`catch` that returns `null`.
</details>

<details>
<summary>Hint: token bucket</summary>

Per key keep `{ tokens, updatedAt }`. On every call add `elapsedSeconds * refillPerSecond`, cap at `burst`, then spend one token if at least one is available. Seconds until the next token: `Math.ceil((1 - tokens) / refillPerSecond)`.
</details>

<details>
<summary>Hint: errors with headers</summary>

`HttpError` accepts `{ headers }`, which the error handler copies onto the response: that is how `Retry-After` reaches a thrown 429.
</details>

<details>
<summary>Hint: logging</summary>

An `onResponse` hook runs after the response was sent and has `reply.statusCode` and `reply.elapsedTime` (milliseconds). Pino omits properties that are `undefined`, which gives you "`user_id` only when known" for free.
</details>

## Stretch goals (not tested)

- Evict idle token buckets so the map cannot grow without bound.
- Add `expires_at` to links and answer `410 Gone` after it.
- Count hits in a separate `hits` table with a timestamp and expose daily totals.
- Move login throttling (failed attempts per email) onto the same limiter.

## Reference solution

`solution/src/` holds only the six files that differ from `starter/src/`. Try the tasks first, then compare. To run the tests against it, save your own work first, then overlay it: `cp -r projects/P5-url-shortener/solution/src/. projects/P5-url-shortener/starter/src/`.
