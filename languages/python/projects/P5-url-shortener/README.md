# P5: URL Shortener

**Modules exercised:** [03 REST API design](../../modules/03-rest-api-design/README.md), [05 Database access](../../modules/05-database-access/README.md), [10 Logging and diagnostics](../../modules/10-logging-and-diagnostics/README.md), [11 Auth and security basics](../../modules/11-auth-and-security-basics/README.md).

## Goal

Build a URL shortener with accounts: users register and log in (scrypt-hashed passwords, HS256 JWTs), create short links, are redirected through them, see their hit counts, and are rate limited with a token bucket. Every request leaves exactly one structured JSON log line that never contains secrets.

## Requirements

| Endpoint | Behaviour |
|----------|-----------|
| `POST /auth/register` | body `{"email","password"}` (email contains `@`, stored lower-cased; password 8-128 chars). `201` `{"id","email"}`. Duplicate email `409` `conflict`. Invalid input `422` `validation_failed` with `fields`. The password is stored only as a salted scrypt hash |
| `POST /auth/login` | `200` `{"token":"<JWT>"}`: HS256, claims `sub` (user id as string), `iat`, `exp` = `iat` + 3600. Wrong password **and** unknown email give the identical `401` `unauthorized` response |
| `POST /links` (auth) | body `{"url","slug"?}`. `url` must be an absolute `http`/`https` URL; `slug` is optional (generated: 7 chars `[A-Za-z0-9]`), otherwise 3-32 chars `[A-Za-z0-9_-]`. `201` `{"slug","url"}` plus `Location: /{slug}`. Duplicate slug (any user) `409` `conflict`. Invalid input `422` |
| `GET /{slug}` (public) | `302` with `Location: <url>`; unknown slug `404` `not_found`. Every redirect increments the link's hit counter atomically |
| `GET /links` (auth) | the caller's links, newest first: `{"items":[{"slug","url","hits","created_at"}],"total","limit","offset"}`. `limit` default 20, clamped to 1-100; `offset` default 0; a non-numeric value is `400` `bad_request` |
| `DELETE /links/{slug}` (auth) | `204`; unknown slug `404`; a link owned by someone else `403` `forbidden` (and it survives) |
| Auth | `Authorization: Bearer <token>`. Missing, malformed, expired, wrongly signed or `alg: none` tokens: `401` `unauthorized` |
| Rate limit | token bucket per authenticated user on `POST /links`: `rate_burst` tokens (default 5), refilled at `rate_refill_per_second` (default 1.0). Empty bucket: `429` `rate_limited` with `Retry-After` (whole seconds, at least 1) |
| Access log | one JSON line per request to `Config.log_sink`: `request_id`, `method`, `path` (no query string), `status`, `duration_ms`, and `user_id` only when the caller is known. Passwords, tokens and the `Authorization` header are never logged |
| Request id | echo `X-Request-ID` when it is 1-64 characters of `[A-Za-z0-9._-]`, else generate one; always send it back as a response header and log it |

All errors use the shared envelope `{"error":{"code","message","fields"?}}` (already implemented in `errors.py`). The slugs `auth` and `links` are reserved (`422`), because they are the first path segment of other routes.

Configuration (`Config`; `python -m p5_url_shortener` reads the environment):

| Variable | Default | Meaning |
|----------|---------|---------|
| `ADDR` | `127.0.0.1:8080` | listen address |
| `JWT_SECRET` | none, **required** (at least 32 characters) | HMAC key; the process refuses to start without it |
| `DB_PATH` | `shortener.db` | SQLite file |
| `RATE_BURST`, `RATE_REFILL_PER_SECOND` | `5`, `1.0` | token bucket |

`Config.clock` (time source of the rate limiter), `Config.log_sink` and `Config.scrypt_n` (hash cost) are injectable so tests are fast and deterministic.

## Run

```bash
pytest projects/P5-url-shortener/tests      # tests (from languages/python)
cd projects/P5-url-shortener/starter
JWT_SECRET=change-me-change-me-change-me-0123 ADDR=127.0.0.1:8080 python -m p5_url_shortener
```

```bash
curl -s -X POST localhost:8080/auth/register -H 'content-type: application/json' \
  -d '{"email":"ann@example.com","password":"s3cret-pass"}'
TOKEN=$(curl -s -X POST localhost:8080/auth/login -H 'content-type: application/json' \
  -d '{"email":"ann@example.com","password":"s3cret-pass"}' | python -c 'import sys,json;print(json.load(sys.stdin)["token"])')
curl -s -X POST localhost:8080/links -H "authorization: Bearer $TOKEN" -H 'content-type: application/json' \
  -d '{"url":"https://example.com/docs","slug":"docs"}'
curl -i localhost:8080/docs                                  # 302, Location: https://example.com/docs
curl -s localhost:8080/links -H "authorization: Bearer $TOKEN"
```

Ctrl-C and SIGTERM stop the server gracefully (uvicorn); the access log lines go to stdout.

## Acceptance criteria (tests)

The tests use FastAPI's `TestClient` (redirects are not followed), a temporary SQLite file, a fake clock and an in-memory log sink.

| Criterion | Tests (`tests/test_p5_shortener.py`) |
|-----------|--------------------------------------|
| registration | `test_register_returns_201_and_never_echoes_the_password`, `test_register_rejects_invalid_input_with_422`, `test_register_twice_with_the_same_email_is_409`, `test_password_is_stored_as_a_salted_scrypt_hash` |
| login and JWT | `test_login_returns_an_hs256_jwt_valid_for_one_hour`, `test_unknown_user_and_wrong_password_get_the_same_401` |
| authentication | `test_protected_routes_reject_missing_or_bad_tokens_with_401` |
| creating links | `test_create_link_with_a_chosen_slug_returns_201`, `test_generated_slug_has_seven_alphanumeric_characters`, `test_url_must_be_an_absolute_http_or_https_url`, `test_slug_must_be_3_to_32_url_safe_characters`, `test_duplicate_slug_is_409_even_for_another_user` |
| redirect and hits | `test_short_link_redirects_with_302_without_authentication`, `test_unknown_slug_is_404_with_the_error_envelope`, `test_every_redirect_increments_the_hit_counter` |
| listing | `test_list_shows_only_the_callers_links`, `test_list_limit_is_clamped_and_bad_values_are_400` |
| deleting | `test_owner_can_delete_a_link_and_it_stops_redirecting`, `test_deleting_someone_elses_link_is_403_and_keeps_the_link` |
| rate limit | `test_creating_links_beyond_the_burst_is_429_with_retry_after`, `test_tokens_refill_as_time_passes`, `test_each_user_has_their_own_bucket` |
| structured logging | `test_every_request_logs_one_json_line_with_the_standard_fields`, `test_user_id_is_logged_only_when_the_caller_is_known`, `test_passwords_and_tokens_never_reach_the_log`, `test_request_id_is_echoed_when_safe_and_generated_otherwise` |

## What to implement

The starter has the package layout, `Config`, the error envelope (`errors.py`), the SQLite schema (`db.py`), the entry point and a `create_app` that only installs the error handlers. Every `TODO` is yours:

| File | Work |
|------|------|
| `starter/p5_url_shortener/models.py` | constraints on the request models (email, password length, URL scheme, slug pattern) |
| `starter/p5_url_shortener/security.py` | `hash_password`, `verify_password`, `issue_token`, `decode_token` |
| `starter/p5_url_shortener/store.py` | the SQL: users and links, atomic hit counting, paging |
| `starter/p5_url_shortener/rate_limit.py` | `RateLimiter.try_acquire` |
| `starter/p5_url_shortener/access_log.py` | `install_access_log` middleware |
| `starter/p5_url_shortener/app.py` | wiring, the auth dependency, the rate-limit dependency and all routes |

Tests import `p5_url_shortener` from `starter/`; the solution is overlaid on the same files. Table names and columns are in `db.py` (`users.password_hash` is checked by a test).

<details>
<summary>Hints</summary>

- Start with `store.py` and `security.py`: both are plain functions you can try in a REPL. Always bind SQL parameters with `?`, never format them into the string.
- `sqlite3.IntegrityError` on `INSERT` is how a duplicate email or slug shows up; translate it into `EmailTakenError` / `SlugTakenError` with `raise ... from err`.
- `UPDATE links SET hits = hits + 1 WHERE slug = ? RETURNING url` increments and reads in one statement (no race between two readers).
- `hashlib.scrypt(password.encode(), salt=salt, n=n, r=8, p=1, dklen=32)`; store `n`, `r`, `p` and the salt next to the hash so verification does not depend on current settings. Compare with `hmac.compare_digest`, never `==`.
- Verify a dummy hash when the email is unknown, so both login failures cost the same time.
- `jwt.decode(token, secret, algorithms=["HS256"], options={"require": ["sub", "exp"]})`: always pass `algorithms`, otherwise the token chooses its own algorithm. Catch `jwt.InvalidTokenError` (expired and bad signatures are subclasses).
- `HTTPBearer(auto_error=False)` gives you the credentials or `None` so you can raise your own `401` envelope. A dependency can store the user id in `request.state.user_id` for the access log.
- A token bucket needs only `tokens` and `last_refill` per key: refill by `elapsed * rate`, cap at `burst`, take one token if at least 1 is there, else the wait is `(1 - tokens) / rate`. Guard the dict with a lock: sync endpoints run in a thread pool.
- Declare `GET /{slug}` **last**, otherwise it shadows `GET /links`. Pydantic: `Field(pattern=...)`, `AfterValidator`, `urllib.parse.urlsplit` for the URL checks.
- `@app.middleware("http")` receives `request` and `call_next`; measure with `time.perf_counter()` and log in a `finally` block. Build the log record from named fields only; never dump headers or bodies.
- Use `def` (not `async def`) for routes that call `sqlite3` or scrypt: FastAPI runs them in a thread pool instead of blocking the event loop.
- `secrets.choice` (not `random`) for generated slugs; retry a few times on a collision.

</details>

## Stretch goals (not tested)

- Per-IP rate limiting on `/auth/login` to slow password guessing; an account lockout with back-off.
- Link expiry (`expires_at`) answered with `410 Gone`, and a click log table with referrer and day buckets.
- Replace the in-process bucket with a SQLite-backed one and explain what changes with several worker processes.
- Run the same store code against PostgreSQL with `psycopg` when `DATABASE_URL` is set (module 05).
- Short-lived access tokens plus refresh tokens, or rotate `JWT_SECRET` with a `kid` header.

## Reference solution

[`solution/`](./solution/) holds the files that differ from the starter (`models.py`, `security.py`, `store.py`, `rate_limit.py`, `access_log.py`, `app.py`). Try the project first, then compare.
