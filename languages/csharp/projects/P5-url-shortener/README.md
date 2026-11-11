# P5 - URL Shortener

A small multi-user URL shortener: register, log in with a JWT, create short links, redirect anonymously, count hits, rate-limit link creation, and write one structured JSON log line per request.

**Modules exercised:** [03 REST API design](../../modules/03-rest-api-design/README.md), [05 Database access](../../modules/05-database-access/README.md), [10 Logging and diagnostics](../../modules/10-logging-and-diagnostics/README.md), [11 Auth and security basics](../../modules/11-auth-and-security-basics/README.md).

## Requirements

Errors are `application/problem+json` (RFC 9457) with the status codes below (the C# equivalent of the shared JSON error envelope). Validation failures are **422** `ValidationProblemDetails`.

| Endpoint | Auth | Behaviour |
|----------|------|-----------|
| `POST /auth/register` `{"email","password"}` | - | email must contain `@` (stored lower-cased), password >= 8 chars, else 422; duplicate email 409; success `201 {"id","email"}`; password stored as salted PBKDF2 hash |
| `POST /auth/login` | - | `200 {"token":"<JWT>"}` (HS256, `exp` = 1 h after issue, claims `sub`, `email`); unknown user and wrong password return the **same** 401 |
| `POST /links` `{"url","slug"?}` | JWT | url must be absolute `http`/`https`, else 422; slug optional: given -> 3-32 chars of `[A-Za-z0-9_-]` and not `links`/`auth` (else 422); omitted -> 7 random `[A-Za-z0-9]` chars; taken slug -> 409; success `201 {"slug","url"}` |
| `GET /{slug}` | public | `302` + `Location`, hit counter +1; unknown 404 |
| `GET /links` | JWT | `200 {"items":[{"slug","url","hits"}],"total":N}` with the caller's links only, oldest first |
| `DELETE /links/{slug}` | JWT | owner: 204; another user: 403; unknown: 404 |
| missing/invalid/expired token | | 401 |

- **Rate limit:** token bucket **per user** on `POST /links` only: capacity `RateLimitBurst`, refilled `RateLimitRefillPerSecond` tokens per second. Over the limit -> `429` + `Retry-After` (whole seconds, >= 1).
- **Logging:** one JSON line per request to an `ILogLineSink` (console by default): `request_id`, `method`, `path` (no query string), `status`, `duration_ms`, and `user_id` when authenticated. `X-Request-ID` is echoed (or generated) in the response. Passwords and tokens never appear in any line.
- **JWT validation:** issuer, audience, lifetime (no clock skew), signature and algorithm are all validated.

## Configuration

Section `Shortener`; env vars use `__`. The secret is required (startup fails with a clear message if it is missing or shorter than 32 characters).

| Env var | Default |
|---------|---------|
| `ADDR` | `127.0.0.1:8080` |
| `Shortener__JwtSecret` | **required** |
| `Shortener__ConnectionString` | `Data Source=shortener.db` |
| `Shortener__Pbkdf2Iterations` | `600000` |
| `Shortener__RateLimitBurst` | `10` |
| `Shortener__RateLimitRefillPerSecond` | `1` |

## Run and test

All commands run from `languages/csharp/`.

```bash
ADDR=127.0.0.1:8080 Shortener__JwtSecret='dev-only-secret-dev-only-secret-0001' \
  dotnet run --project projects/P5-url-shortener/starter

curl -s -X POST localhost:8080/auth/register -H 'Content-Type: application/json' \
  -d '{"email":"ada@example.com","password":"correct horse"}'
TOKEN=$(curl -s -X POST localhost:8080/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"ada@example.com","password":"correct horse"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["token"])')
curl -s -X POST localhost:8080/links -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"url":"https://example.com/docs","slug":"docs"}'
curl -si localhost:8080/docs | head -3
curl -s localhost:8080/links -H "Authorization: Bearer $TOKEN"

dotnet test projects/P5-url-shortener/tests
```

Real output of those calls with the reference solution:

```text
{"id":1,"email":"ada@example.com"}
{"slug":"docs","url":"https://example.com/docs"}
HTTP/1.1 302 Found
Content-Length: 0
Date: ...
{"items":[{"slug":"docs","url":"https://example.com/docs","hits":1}],"total":1}
```

and the log lines on the server console:

```text
{"request_id":"5745c313...","method":"POST","path":"/auth/register","status":201,"duration_ms":212.423}
{"request_id":"b17fc869...","method":"POST","path":"/links","status":201,"duration_ms":38.084,"user_id":1}
{"request_id":"b08dff37...","method":"GET","path":"/docs","status":302,"duration_ms":11.978}
```

The tests use `WebApplicationFactory<Program>` per test with a private shared-cache in-memory SQLite database, a `FakeTimeProvider`, a list-backed `ILogLineSink` and 1 000 PBKDF2 iterations. One test reads `users.password_hash` straight from the database; the schema lives in the provided `Database.cs`.

## What to implement

The starter compiles and wires everything (`Program.cs`: DI, JWT validation, middleware order, routes; `Database.cs`: schema; options, DTOs). These files contain `TODO(P5)` stubs:

| File | Task |
|------|------|
| `PasswordHasher.cs` | PBKDF2-HMAC-SHA256, random salt, constant-time verify |
| `TokenService.cs` | issue the HS256 JWT using the injected `TimeProvider` |
| `UserStore.cs`, `LinkStore.cs` | parameterised ADO.NET queries, duplicate detection, atomic hit counter |
| `AuthEndpoints.cs`, `LinkEndpoints.cs` | validation, status codes, ownership rules, slug generation |
| `RateLimiting.cs` | the per-user token bucket policy and the 429 response |
| `RequestLoggingMiddleware.cs` | request id + one JSON line per request |

## Acceptance criteria -> tests (`tests/ShortenerTests.cs`)

| Criterion | Test |
|-----------|------|
| register 201, lower-cased email, no password in body | `Register_returns_201_with_the_new_user` |
| register validation 422 | `Register_rejects_invalid_input_with_422` |
| duplicate email 409 | `Register_rejects_a_duplicate_email_with_409` |
| stored hashed | `Password_is_stored_hashed_not_in_plain_text` |
| HS256 JWT, 1 h | `Login_returns_an_HS256_jwt_that_expires_after_one_hour` |
| same 401 for unknown user / bad password | `Unknown_user_and_wrong_password_get_the_same_401` |
| expiry and signature enforced | `Token_is_rejected_after_it_expires`, `Token_signed_with_another_secret_is_rejected` |
| links need auth | `Creating_a_link_requires_authentication`, `Deleting_requires_auth_and_an_unknown_slug_is_404` |
| generated slug | `Link_without_slug_gets_a_generated_7_character_slug` |
| url / slug validation | `Link_url_must_be_an_absolute_http_or_https_url`, `Link_slug_must_be_3_to_32_url_safe_characters` |
| duplicate slug 409 | `Custom_slug_is_used_and_a_duplicate_slug_returns_409_even_for_another_user` |
| redirect and 404 | `Public_slug_redirects_with_302_and_an_unknown_slug_is_404` |
| hit counter | `Every_redirect_increments_the_hit_counter` |
| list own links | `Listing_returns_only_the_callers_links` |
| owner-only delete | `Owner_can_delete_a_link_and_it_stops_redirecting`, `Another_user_cannot_delete_the_link_and_gets_403` |
| token bucket, 429 + `Retry-After`, per user | `Creating_links_beyond_the_burst_returns_429_with_retry_after`, `The_bucket_is_per_user` |
| one JSON log line with required fields | `Each_request_writes_one_json_line_with_the_required_fields` |
| request id | `Request_id_is_echoed_or_generated_and_matches_the_log` |
| `user_id` only when authenticated | `User_id_is_logged_only_for_authenticated_requests` |
| no secrets in logs | `Passwords_and_tokens_never_appear_in_the_log` |

## Suggested steps and hints

1. `PasswordHasher`, `UserStore`, then `register`/`login` (needs `TokenService`).
2. `LinkStore` and the four link endpoints.
3. The rate limiter.
4. The logging middleware.

<details>
<summary>Hint: password hashing</summary>

`Rfc2898DeriveBytes.Pbkdf2(password, salt, iterations, HashAlgorithmName.SHA256, 32)` with `RandomNumberGenerator.GetBytes(16)`. Store `iterations`, salt and key together so you can verify later even if the setting changes. Compare with `CryptographicOperations.FixedTimeEquals`, never `==`.
</details>

<details>
<summary>Hint: duplicates and atomic counters</summary>

Let the database enforce uniqueness: catch `SqliteException` with `SqliteErrorCode == 19` (constraint) instead of "select then insert". `UPDATE links SET hits = hits + 1 WHERE slug = @slug RETURNING url` increments and reads in one statement, so concurrent hits cannot be lost.
</details>

<details>
<summary>Hint: JWT</summary>

`SecurityTokenDescriptor` + `new JsonWebTokenHandler().CreateToken(descriptor)` with `SigningCredentials(key, SecurityAlgorithms.HmacSha256)`. Set `IssuedAt`, `NotBefore` and `Expires` from `TimeProvider`. `Program.cs` already validates issuer, audience, lifetime and signature; the claim `sub` is read by `GetUserId()`.
</details>

<details>
<summary>Hint: rate limiter</summary>

`services.AddRateLimiter(o => o.AddPolicy(name, httpContext => RateLimitPartition.GetTokenBucketLimiter(userKey, _ => new TokenBucketRateLimiterOptions { ... })))`. Read the settings from `httpContext.RequestServices` (the factory runs once per partition key). For fractional refill use `TokensPerPeriod = 1` and `ReplenishmentPeriod = 1 / refillPerSecond` seconds. In `OnRejected`, read `MetadataName.RetryAfter` from `context.Lease`.
</details>

<details>
<summary>Hint: logging</summary>

Log in a `finally` after `await next(context)`: authentication has run by then, so `context.User` is populated and `context.Response.StatusCode` is final. Serialize a `Dictionary<string, object?>` with `JsonSerializer`. Use `Request.Path`, not the full URL.
</details>

## Stretch goals (not tested)

- Replace the custom sink with `ILogger` and the built-in JSON console formatter plus a logging scope carrying `request_id`.
- Add `expires_at` to links and a cleanup `BackgroundService`.
- Add a `GET /links/{slug}` endpoint returning the owner's link details.
- Swap SQLite for PostgreSQL by changing the connection and using `NpgsqlConnection` (compile-only, see module 05).

## Reference solution

`solution/` contains only the eight files that differ from `starter/`. Try the tasks first, then compare. To run the tests against the solution without touching the starter, copy `projects/P5-url-shortener` elsewhere and overlay `solution/*` onto `starter/`.
