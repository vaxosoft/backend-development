# P5 - URL Shortener (Go)

**Goal:** a URL shortener with registration and JWT login, persistence in SQLite, per-user token-bucket rate limiting and structured JSON request logs that never leak secrets.

**Modules exercised:** [03 REST API design](../../modules/03-rest-api-design/README.md), [05 Database access](../../modules/05-database-access/README.md), [10 Logging and diagnostics](../../modules/10-logging-and-diagnostics/README.md), [11 Auth and security basics](../../modules/11-auth-and-security-basics/README.md).

## Requirements

Errors use the shared envelope `{"error":{"code","message","fields"?}}` with `Content-Type: application/json`.

| Endpoint | Behaviour |
|----------|-----------|
| `POST /auth/register` `{"email","password"}` | `201 {"id","email"}`; e-mail needs an `@`; password 8-72 bytes (bcrypt ignores the rest) -> `422 validation_failed` with `fields`; duplicate e-mail -> `409 conflict`; password stored only as a bcrypt hash; malformed JSON -> `400 bad_request` |
| `POST /auth/login` | `200 {"token":"<JWT>"}`: HS256, `sub` = user id, `iat`, `exp` = 1 h; unknown user and wrong password give the **same** `401 unauthorized` answer |
| `POST /links` (auth) `{"url","slug"?}` | `201 {"slug","url"}`; `url` absolute http/https with a host; `slug` optional: generated = 7 chars `[A-Za-z0-9]`, custom = 3-32 chars `[A-Za-z0-9_-]` and not `auth`/`links` (reserved by routes) -> `422` with `fields`; taken slug -> `409 conflict` |
| `GET /{slug}` (public) | `302` + `Location`; unknown -> `404 not_found`; every hit increments a counter (atomically) |
| `GET /links` (auth) | caller's links, newest first: `{"items":[{"slug","url","hits"}],"total","limit","offset"}`; `limit` default 20, max 100 (clamped), `offset` default 0; invalid or negative -> `400` |
| `DELETE /links/{slug}` (auth) | `204`; unknown `404`; someone else's link `403 forbidden` |
| Auth | `Authorization: Bearer <jwt>`; missing/invalid/expired token -> `401 unauthorized`. Only HS256 is accepted and `exp` is required |
| Rate limit | token bucket **per user** on `POST /links` (`RateBurst`, `RateRefillPerSecond`) -> `429 rate_limited` + integer `Retry-After` (seconds) |
| Logging | exactly one JSON line per request: `request_id`, `method`, `path`, `status`, `duration_ms`, plus `user_id` when known. `X-Request-ID` is echoed or generated. Passwords, tokens, `Authorization` headers and query strings are never logged |

## Acceptance criteria -> tests

All in [`tests/shortener_test.go`](tests/shortener_test.go) (real HTTP via `httptest.NewServer`, SQLite in a temp dir, bcrypt `MinCost`).

| Criterion | Test |
|-----------|------|
| register 201, no secrets in the response | `TestRegisterCreatesUser` |
| register validation | `TestRegisterValidation` |
| duplicate e-mail | `TestRegisterDuplicateEmailIs409` |
| bcrypt hash at rest | `TestPasswordIsStoredAsBcryptHash` |
| HS256, `exp - iat` = 1 h, `sub` | `TestLoginReturnsHS256TokenValidForOneHour` |
| same 401 for unknown user / wrong password | `TestLoginFailuresAreIndistinguishable` |
| bad tokens (none, garbage, wrong key, expired, `alg:none`, HS384, no `exp`) | `TestInvalidTokensAreRejected` |
| generated slug | `TestCreateLinkGeneratesSevenCharacterSlug` |
| custom slug | `TestCreateLinkAcceptsCustomSlug` |
| URL/slug validation (422 + fields) | `TestCreateLinkValidation` |
| duplicate slug | `TestDuplicateSlugIs409AndKeepsTheOriginal` |
| malformed JSON | `TestMalformedJSONIs400` |
| redirect | `TestRedirectIs302WithLocation`, `TestUnknownSlugIs404` |
| hit counter under concurrency | `TestConcurrentRedirectsCountEveryHit` |
| list own links, pagination | `TestListLinksReturnsOnlyOwnLinksWithHits`, `TestListLinksPaginates` |
| delete / ownership | `TestOwnerCanDeleteLink`, `TestOtherUsersCannotDeleteLink`, `TestDeleteUnknownLinkIs404` |
| rate limit + `Retry-After`, per user | `TestCreateLinkRateLimitReturns429WithRetryAfter`, `TestRateLimitIsPerUser` |
| structured logs | `TestOneJSONLogLinePerRequest`, `TestLogsNeverContainSecrets`, `TestRequestIDIsEchoedOrGenerated` |

## What is given, what you write

Provided in [`starter/`](starter/): `Config`, `New`/routing (`shortener.go`), schema + `openStore` (`db.go`), JSON/error helpers and the per-request `reqInfo` (`helpers.go`), `ConfigFromEnv`, and `cmd/server`.

| File | You implement | Module |
|------|---------------|--------|
| [`store.go`](starter/store.go) | the seven SQL methods (`CreateUser`, `UserByEmail`, `CreateLink`, `LinkBySlug`, `Hit`, `LinksByOwner`, `DeleteLink`) | 05 |
| [`auth.go`](starter/auth.go) | bcrypt helpers, JWT issue/verify, `handleRegister`, `handleLogin` | 11 |
| [`middleware.go`](starter/middleware.go) | `logRequests`, `requireAuth`, `limitCreates` | 10, 11 |
| [`links.go`](starter/links.go) | validation, `newSlug`, and the four link handlers | 03 |

## Run it

From `languages/go/`:

```bash
JWT_SECRET=change-me-please-32-bytes-long ADDR=127.0.0.1:8080 go run ./projects/P5-url-shortener/starter/cmd/server
```

| Variable | Default | Meaning |
|----------|---------|---------|
| `ADDR` | `127.0.0.1:8080` | listen address (Ctrl-C shuts down gracefully) |
| `JWT_SECRET` | (required, >= 16 bytes) | HS256 key |
| `DB_PATH` | `shortener.db` | SQLite file |
| `RATE_BURST` / `RATE_REFILL` | `10` / `1` | bucket size / tokens per second |

```bash
curl -s -X POST localhost:8080/auth/register -d '{"email":"ann@example.com","password":"correct-horse"}'
TOKEN=$(curl -s -X POST localhost:8080/auth/login -d '{"email":"ann@example.com","password":"correct-horse"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["token"])')
curl -s -X POST localhost:8080/links -H "Authorization: Bearer $TOKEN" -d '{"url":"https://go.dev/doc/","slug":"godoc"}'
curl -si localhost:8080/godoc        # 302 Found, Location: https://go.dev/doc/
curl -s localhost:8080/links -H "Authorization: Bearer $TOKEN"
```

## Run the tests

```bash
go test ./projects/P5-url-shortener/tests/                 # fails until you are done
go test -race -count=3 ./projects/P5-url-shortener/tests/  # needs gcc
```

## Suggested steps

1. `store.go` (the tests cannot get far without it); try it from a throw-away `main` or just follow the tests.
2. `hashPassword`/`checkPassword`, `handleRegister`, then `handleLogin`.
3. `issueToken`/`parseToken`, then `requireAuth`.
4. `validateURL`, `validateSlug`, `newSlug`, then the link handlers.
5. `limitCreates`, then `logRequests`.

<details>
<summary>Hints</summary>

- SQLite in-memory databases exist **per connection**; `openStore` limits the pool to one connection, so never keep a `*sql.Rows` open while running another query.
- `UPDATE links SET hits = hits + 1 WHERE slug = ? RETURNING url` is one atomic statement: no read-modify-write race and no transaction needed. `sql.ErrNoRows` means the slug does not exist.
- Unique violations: `isUnique(err)` is provided; map it to `ErrConflict`, and handlers map that to `409`.
- Login: run exactly one `bcrypt` comparison in both cases (use `s.dummyHash` when the user is unknown) so response time does not reveal which e-mails exist.
- JWT: `jwt.ParseWithClaims(tok, &jwt.RegisteredClaims{}, keyFunc, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())`. The key function must return `[]byte`.
- Token bucket: `rate.NewLimiter(rate.Limit(refill), burst)`. `Reserve()` + `Delay()` tells you how long until a token exists; if `Delay() > 0`, `Cancel()` the reservation and answer 429 with `ceil(Delay seconds)` (at least 1).
- Logging: `auth` runs *inside* `logRequests`, so the logger cannot see the user id on the way in. The shared `*reqInfo` pointer stored in the context solves it: `requireAuth` writes `UserID`, the logger reads it after `next.ServeHTTP` returns. Use a `statusRecorder` to learn the status code.
- `slog.Logger.LogAttrs(ctx, slog.LevelInfo, "request", attrs...)` writes one JSON line through the handler built in `New`.
- Slug generation with `crypto/rand`: draw bytes, skip those >= 248 (62 x 4) to avoid modulo bias.

</details>

## Stretch goals (not tested)

- Expiring links, or a click-per-day table.
- `HEAD /{slug}` without counting a hit.
- Swap SQLite for PostgreSQL through `pgx/v5/stdlib` (module 05) and a `DATABASE_URL`.
- Refresh tokens or token revocation.
- Return `429` also on `POST /auth/login` after repeated failures.

## Reference solution

[`_solution/`](_solution/) contains the files that differ (`store.go`, `auth.go`, `middleware.go`, `links.go`). To try it (from `languages/go/`):

```bash
cp projects/P5-url-shortener/_solution/*.go projects/P5-url-shortener/starter/
go test ./projects/P5-url-shortener/tests/
```
