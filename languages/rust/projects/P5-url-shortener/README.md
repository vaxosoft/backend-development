# P5: URL Shortener

**Time:** 3-4 hours | **Difficulty:** Intermediate-Advanced | **Modules:** [03 REST API design](../../modules/03-rest-api-design/README.md), [05 Database access](../../modules/05-database-access/README.md), [10 Logging and diagnostics](../../modules/10-logging-and-diagnostics/README.md), [11 Auth and security basics](../../modules/11-auth-and-security-basics/README.md)

## Goal
A multi-user URL shortener: register, log in, create short links, share them (public redirect), see your hit counts, delete your own links. It adds the production concerns a real service needs: hashed passwords, JWTs, a per-user rate limit and one structured JSON log line per request that never contains a secret.

## Requirements

| Endpoint | Auth | Behaviour |
|----------|------|-----------|
| `POST /auth/register` `{"email","password"}` | no | `201 {"id","email"}`. Email is trimmed and lowercased, must contain `@`; password at least 8 characters (else `422` with `fields`). Duplicate email `409 conflict`. The password is stored as an Argon2id hash |
| `POST /auth/login` | no | `200 {"token":"<JWT>"}`: HS256, claims `sub` (user id) and `exp` (1 hour). Unknown email and wrong password give the **same** `401` body |
| `POST /links` `{"url","slug"?}` | Bearer | `201 {"slug","url"}`. `url` absolute `http`/`https` (stored in normalised form); `slug` 3-32 of `[A-Za-z0-9_-]` and not `links`, otherwise 7 random `[A-Za-z0-9]`. Invalid -> `422`; taken slug -> `409 conflict` |
| `GET /{slug}` | no | `302` with `Location`; unknown `404`; each visit adds one hit |
| `GET /links` | Bearer | `200 {"items":[{"slug","url","hits"}]}`, only the caller's links, oldest first |
| `DELETE /links/{slug}` | Bearer | `204`; unknown `404`; somebody else's link `403` |

Cross-cutting rules:
- Errors use the envelope `{"error":{"code","message","fields"?}}`: `bad_request` 400 (malformed JSON), `unauthorized` 401 (missing, invalid or expired token), `forbidden` 403, `not_found` 404, `conflict` 409, `validation_failed` 422, `rate_limited` 429, `internal` 500 (never leaks details).
- **Rate limit:** a token bucket per authenticated user on `POST /links` only (`rate_burst`, `rate_refill_per_sec`). Over the limit: `429 rate_limited` with `Retry-After` (whole seconds, at least 1).
- **Logging:** exactly one JSON line per request, written to the configured sink: `request_id`, `method`, `path` (no query string), `status`, `duration_ms`, and `user_id` when the request carried a valid token. `X-Request-ID` is echoed (or generated) and set on the response. Passwords and tokens are never logged.

Configuration (`src/main.rs`): `ADDR` (default `127.0.0.1:8080`), `DATABASE_URL` (default `sqlite::memory:`; for a file use `sqlite://shortener.db`), `JWT_SECRET`, `RATE_BURST`, `RATE_REFILL_PER_SEC`.

## Your work
Everything compiles and serves, but these files contain `todo!()`:

| File | What to write |
|------|---------------|
| `starter/src/auth.rs` | `hash_password`, `verify_password`, `issue_token`, `verify_token`, `bearer_user` |
| `starter/src/store.rs` | the seven SQL queries (`sqlx::query`, `query_as`, `query_scalar`) |
| `starter/src/handlers.rs` | the six handlers, validation and slug generation |
| `starter/src/limiter.rs` | the token bucket |
| `starter/src/logging.rs` | the request-logging middleware (the starter only passes requests through) |

Provided: `Config`/`build_app` wiring, `ApiError` + `ApiJson` (JSON rejections in the envelope), the SQLite schema (`db.rs`), DTOs (`models.rs`), and `jwt_crypto.rs` (`jsonwebtoken` 11 needs a crypto backend; this workspace installs a small HS256 one). In your own project enable the crate's `rust_crypto` feature instead.

## Run the tests
From `languages/rust/`:
```bash
cargo test -p p5-url-shortener
```
Each test builds its own app on port 0 with an in-memory SQLite database (one temp file for the "stored as a hash" test), a capturing log sink and a cheap Argon2 setting (`hash_memory_kib = 64`), so tests are isolated and the suite takes well under a second once solved.

## Acceptance criteria -> tests

| Criterion | Test |
|-----------|------|
| Register, normalised email, no password echoed | `register_creates_a_user_and_never_echoes_the_password` |
| Register validation / duplicates / bad JSON | `register_rejects_bad_email_and_short_password_with_field_errors`, `registering_the_same_email_twice_is_a_conflict`, `malformed_json_is_a_400_with_the_error_envelope` |
| Password stored hashed | `passwords_are_stored_as_argon2_hashes` |
| JWT HS256, 1 hour | `login_returns_an_hs256_jwt_that_expires_in_one_hour` |
| Same 401 for unknown user and wrong password | `unknown_user_and_wrong_password_are_indistinguishable` |
| Token required, expired rejected | `protected_routes_reject_missing_garbage_and_expired_tokens` |
| Create link, generated slug | `create_link_with_a_custom_slug`, `create_link_without_a_slug_generates_seven_alphanumeric_characters` |
| URL and slug rules | `only_absolute_http_and_https_urls_are_accepted`, `slugs_must_be_3_to_32_url_safe_characters_and_not_reserved` |
| Duplicate slug 409 | `a_taken_slug_is_a_409_even_for_another_user` |
| Redirect, 404, hit counter | `visiting_a_slug_redirects_with_302_and_needs_no_login`, `unknown_slug_is_a_404_with_the_error_envelope`, `every_redirect_counts_a_hit_and_list_shows_only_own_links` |
| Delete rules | `owner_can_delete_a_link_and_it_stops_redirecting`, `deleting_needs_login_and_ownership_and_an_existing_slug` |
| Token bucket | `creating_links_beyond_the_burst_is_a_429_with_retry_after`, `the_rate_limit_is_per_user`, `tokens_refill_over_time` |
| Structured logs | `every_request_writes_one_json_line_with_the_standard_fields`, `authenticated_requests_log_the_user_id`, `request_id_is_echoed_or_generated_and_matches_the_log`, `passwords_tokens_and_query_strings_never_appear_in_logs` |

## Try it by hand
```bash
JWT_SECRET=change-me-to-a-long-random-string ADDR=127.0.0.1:8080 cargo run -p p5-url-shortener
```
```bash
curl -s -X POST localhost:8080/auth/register -H 'Content-Type: application/json' \
  -d '{"email":"ada@example.com","password":"correct horse"}'
TOKEN=$(curl -s -X POST localhost:8080/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"ada@example.com","password":"correct horse"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["token"])')
curl -s -X POST localhost:8080/links -H 'Content-Type: application/json' \
  -H "Authorization: Bearer $TOKEN" -d '{"url":"https://example.com/docs","slug":"docs"}'
curl -si localhost:8080/docs | head -n 3
curl -s localhost:8080/links -H "Authorization: Bearer $TOKEN"
```
```text
{"id":1,"email":"ada@example.com"}
{"slug":"docs","url":"https://example.com/docs"}
HTTP/1.1 302 Found
location: https://example.com/docs
x-request-id: <random uuid>
{"items":[{"slug":"docs","url":"https://example.com/docs","hits":1}]}
```
Each request also prints a line like `{"duration_ms":2,"method":"GET","path":"/links","request_id":"...","status":200,"user_id":1}` on the server's stdout.

## Suggested steps
1. `auth.rs` + `register`/`login`: the tests for registration and login turn green.
2. `store.rs` + `create_link`/`redirect`/`list_links`/`delete_link`.
3. `limiter.rs` and wire it into `create_link`.
4. `logging.rs`.

<details>
<summary>Hint: Argon2 and the async runtime</summary>

`Argon2::new(Algorithm::Argon2id, Version::V0x13, Params::new(memory_kib, 2, 1, None)?)` then `hash_password(bytes)` (trait `PasswordHasher`) picks a random salt; `.to_string()` is the PHC string. Verification parses the stored string with `PasswordHash::new` and uses `PasswordVerifier::verify_password`; the cost is read from the hash. Both take hundreds of milliseconds: call them inside `tokio::task::spawn_blocking`.
</details>

<details>
<summary>Hint: JWT</summary>

`Header::new(Algorithm::HS256)`, a `#[derive(Serialize, Deserialize)]` claims struct with `sub: String, exp: u64`, `encode` / `decode`. Build the `Validation` yourself: `Validation::new(Algorithm::HS256)`, `leeway = 0` (the default is 60 s), `set_required_spec_claims(&["exp", "sub"])`. A negative `ttl_secs` must produce an already-expired token, so use `saturating_add_signed` on the epoch seconds.
</details>

<details>
<summary>Hint: SQL</summary>

Unique violations: `err.as_database_error().is_some_and(|e| e.is_unique_violation())`. Counting a hit atomically: `UPDATE links SET hits = hits + 1 WHERE slug = ? RETURNING url` with `query_scalar(...).fetch_optional`. In sqlx 0.9 `query` only accepts `&'static str` SQL: values must go through `.bind`.
</details>

<details>
<summary>Hint: 302, URL validation, slugs</summary>

`axum::response::Redirect::temporary` answers 307. Build `(StatusCode::FOUND, [(header::LOCATION, url)])` instead. `reqwest::Url::parse` is available (reqwest re-exports `url::Url`); check `scheme()` and `host_str()`. For the slug use `rand::rng()` with `RngExt::random_range` over a byte alphabet.
</details>

<details>
<summary>Hint: token bucket</summary>

State per user: `tokens: f64` and `last_refill: Instant`. On each check: `tokens = min(burst, tokens + elapsed * refill_per_sec)`; if `tokens >= 1` take one, else the wait is `(1 - tokens) / refill_per_sec` seconds. Round `Retry-After` up.
</details>

<details>
<summary>Hint: logging middleware</summary>

`axum::middleware::from_fn_with_state` is already wired. Read what you need from the `Request` (method, `uri().path()`, headers) before `next.run(request).await`, then build the line with `serde_json::json!` (it escapes untrusted values for you) and call `(state.config.log_sink)(line)`.
</details>

## Stretch goals (not tested)
- Verify against a dummy hash when the email is unknown, so login timing does not reveal which emails exist.
- Route the JSON lines through `tracing` + `tracing-subscriber`'s JSON formatter and a `TraceLayer` instead of the hand-written sink.
- Expiring links, click timestamps, and `GET /links?limit=&offset=` with the standard list envelope.
- Evict idle token buckets so the limiter's memory stays bounded.
- Switch `DATABASE_URL` to PostgreSQL (module 05 shows the swap).

## Reference solution
[`solution/`](./solution/) holds only the five files that differ. To try it, overlay it (copy your own `starter/` first if you want to keep your work):
```bash
cp -r projects/P5-url-shortener/solution/. projects/P5-url-shortener/starter/
cargo test -p p5-url-shortener
```
