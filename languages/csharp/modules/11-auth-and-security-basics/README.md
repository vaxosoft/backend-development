# 11. Authentication and Security Basics in C#

> Estimated time: 2h | Difficulty: Intermediate | Prerequisites: 02 (HTTP server, middleware, DI), 03 (ProblemDetails), 05 (parameterized SQL)

## Learning Outcomes
After this module you can:
- [ ] Hash and verify passwords with PBKDF2, a per-user salt and a constant-time comparison
- [ ] Issue and validate HS256 JWTs with every validation parameter switched on
- [ ] Protect Minimal API endpoints with `RequireAuthorization`, roles and named policies
- [ ] Explain and prevent SQL injection and XSS, and know when CSRF matters
- [ ] Rate-limit an endpoint per user with a token bucket and answer `429` + `Retry-After`

## 1. Concept Overview
**Authentication** answers "who are you?" (password -> token), **authorization** answers "what may you do?" (role/policy/ownership checks). Passwords are never stored: you store a slow, salted hash so a stolen database does not reveal them. After login the server hands out a signed, short-lived credential (a JWT here; a cookie for browser apps) that the client sends on every request.

Security basics are mostly *refusing to trust input*: use query parameters instead of string concatenation (SQL injection), encode on output (XSS), validate tokens completely (signature, issuer, audience, lifetime, algorithm), compare secrets in constant time, and slow abusers down with rate limits. TLS protects all of it in transit: in production terminate HTTPS at the reverse proxy or Kestrel and send `Strict-Transport-Security`.

```
client --POST /auth/login--> verify PBKDF2 hash --> JWT (sub, role, exp)
client --Bearer JWT--------> UseAuthentication -> UseAuthorization (policy) -> UseRateLimiter -> endpoint
```

## 2. How C# Approaches It
| Need | In the box | Notes |
|------|-----------|-------|
| Password hashing | `Rfc2898DeriveBytes.Pbkdf2` | static, span-friendly; use SHA-256 and 600k+ iterations |
| Constant-time compare | `CryptographicOperations.FixedTimeEquals` | never `==`/`SequenceEqual` on secrets |
| Randomness for salts/keys | `RandomNumberGenerator.GetBytes` | never `System.Random` |
| JWT | `JsonWebTokenHandler` (`Microsoft.IdentityModel.JsonWebTokens`) + `AddJwtBearer` | the only extra package: `Microsoft.AspNetCore.Authentication.JwtBearer` |
| Authorization | `RequireAuthorization()`, `AddAuthorizationBuilder().AddPolicy(...)` | roles are just a claim type; policies compose requirements |
| Rate limiting | `AddRateLimiter` / `UseRateLimiter` / `.RequireRateLimiting("name")`, `System.Threading.RateLimiting` | fixed/sliding window, token bucket, concurrency |
| Cookies / CSRF | `AddCookie`, `AddAntiforgery` | for browser sessions only; bearer tokens in a header are not CSRF-prone |
| XSS | `WebUtility.HtmlEncode`; Razor encodes by default | JSON APIs return data, the *client* must not inject it as HTML |

Idiomatic: configure everything in the DI container (`AddAuthentication().AddJwtBearer()`), declare rules next to the endpoint (`.RequireAuthorization("AdminOnly")`), and keep the middleware order `UseAuthentication` -> `UseAuthorization` -> `UseRateLimiter`. Non-idiomatic: hand-parsing the `Authorization` header, rolling your own crypto, or disabling `TokenValidationParameters` flags "to make it work". ASP.NET Core Identity offers `PasswordHasher<TUser>`; this module builds the primitive by hand so you see every parameter.

## 3. Guided Examples
All commands run from `languages/csharp/`. Examples are dispatched by name: `dotnet run --project modules/11-auth-and-security-basics/examples -- <name>`.

### 3.1 Password hashing
- **Goal:** salted PBKDF2 with a self-describing record and constant-time verification
- **Run:** `dotnet run --project modules/11-auth-and-security-basics/examples -- 01-password-hashing`
- **Code:** [01_PasswordHashing.cs](examples/01_PasswordHashing.cs)
- **Walkthrough:** the stored value `pbkdf2-sha256$600000$salt$hash` carries its own algorithm and iteration count so you can raise the cost later and still verify old records. `Verify` re-derives with the *stored* parameters and compares with `FixedTimeEquals`; malformed records return `false` instead of throwing. The last line is a published PBKDF2 known-answer vector.
  ```csharp
  var hash = Rfc2898DeriveBytes.Pbkdf2(password, salt, Iterations, HashAlgorithmName.SHA256, HashSize);
  return CryptographicOperations.FixedTimeEquals(actual, expected);
  ```
- **Expected output:**
  ```
  format: pbkdf2-sha256$600000$<salt>$<hash>
  same password, different stored values: True
  verify correct password: True
  verify wrong password: False
  verify garbage record: False
  known answer: 120fb6cffcf8b32c43e7225256c4f837a86548c92ccc35480805987cb70be17b
  ```

### 3.2 JWT issue and validation
- **Goal:** create an HS256 token and see which checks reject which attacks
- **Run:** `dotnet run --project modules/11-auth-and-security-basics/examples -- 02-jwt`
- **Code:** [02_Jwt.cs](examples/02_Jwt.cs)
- **Walkthrough:** `ValidAlgorithms` pins HS256 (blocks `alg` confusion), `ClockSkew = Zero` makes `exp` exact, issuer/audience stop tokens minted for another service. `JsonWebTokenHandler` leaves claim names as-is (`sub`, `role`), so set `NameClaimType`/`RoleClaimType` accordingly. A forged payload fails with a signature exception: the key is not found/verified.
- **Expected output:**
  ```
  header: {"alg":"HS256","typ":"JWT"}
  claims: aud,exp,iat,iss,nbf,role,sub
  valid          valid=True  sub=42 admin=True
  expired        valid=False SecurityTokenExpiredException
  wrong key      valid=False SecurityTokenSignatureKeyNotFoundException
  tampered       valid=False SecurityTokenSignatureKeyNotFoundException
  wrong audience valid=False SecurityTokenInvalidAudienceException
  ```

### 3.3 Injection and XSS
- **Goal:** show the exploit, then the two standard defences
- **Run:** `dotnet run --project modules/11-auth-and-security-basics/examples -- 03-injection-and-xss`
- **Code:** [03_InjectionAndXss.cs](examples/03_InjectionAndXss.cs)
- **Walkthrough:** interpolating `' OR '1'='1` into SQL returns every row; `command.Parameters.AddWithValue("@email", ...)` sends it as data, so nothing matches. `WebUtility.HtmlEncode` turns markup into inert text (do it at output time, for the context you output into).
- **Expected output:**
  ```
  concatenated SQL leaks: Ada, Bob
  parameterized rows: 0
  encoded output: &lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;
  ```

### 3.4 Token-bucket rate limiter
- **Goal:** partition limits by user and read `Retry-After`
- **Run:** `dotnet run --project modules/11-auth-and-security-basics/examples -- 04-rate-limiter`
- **Code:** [04_RateLimiterDemo.cs](examples/04_RateLimiterDemo.cs)
- **Walkthrough:** `PartitionedRateLimiter.Create` gives each key its own bucket (burst 3, refill 1 per 10 s). The 4th request from `ada` is refused with `RetryAfter` metadata; `bob` has his own bucket. Automatic replenishment is off only to make the output deterministic.
- **Expected output:**
  ```
  ada  allowed=True
  ada  allowed=True
  ada  allowed=True
  ada  allowed=False retry-after=10s
  bob  allowed=True
  ```

### 3.5 Task Tracker: auth, roles and rate limit over HTTP
- **Goal:** put it together: register, login, bearer auth, admin policy, throttled `POST /tasks`
- **Run:** `ADDR=127.0.0.1:8080 dotnet run --project modules/11-auth-and-security-basics/examples -- 05-tracker-auth-api` (Ctrl-C to stop; `JWT_SECRET`, `ADMIN_EMAIL` override the dev defaults)
- **Code:** [05_TrackerAuthApi.cs](examples/05_TrackerAuthApi.cs)
- **Walkthrough:** login returns the same `401` for an unknown user and a wrong password (no account enumeration). `AddJwtBearer` plugs the validation parameters into the pipeline; `/admin/stats` requires the `AdminOnly` policy (`RequireRole("admin")`); the rate-limit partition key is the authenticated user name, which is why `UseRateLimiter` comes after `UseAuthentication`. Errors are `application/problem+json` (RFC 9457) with the status codes from the shared REST conventions (422 for validation). A JwtBearer challenge itself has an empty body.
  ```csharp
  app.MapPost("/tasks", (TaskInput body) => ...).RequireAuthorization().RequireRateLimiting("create-task");
  ```
- **Expected output** (`B=http://127.0.0.1:8080`; `$TOKEN`/`$ADMIN` come from `/auth/login`, e.g. `curl -s -X POST $B/auth/login -H 'Content-Type: application/json' -d '{"email":"ada@example.com","password":"s3cret-pass"}' | sed 's/.*"token":"\([^"]*\)".*/\1/'`; `traceId` omitted):
  ```
  POST /auth/register ada@example.com      -> {"id":1,"email":"ada@example.com"} [201]
  POST /auth/register admin@example.com    -> {"id":2,"email":"admin@example.com"} [201]
  POST /auth/register ada again            -> 409
  POST /auth/register bad email/password   -> 422 {"...","errors":{"credentials":["email must contain @ and password must have 8+ characters"]}}
  POST /auth/login wrong password          -> 401
  POST /auth/login unknown user            -> 401
  GET  /me (no token)                      -> 401
  GET  /me (ada)                           -> {"id":"1","role":"user"} [200]
  GET  /me (token + "x")                   -> 401
  GET  /admin/stats (ada)                  -> 403
  GET  /admin/stats (admin)                -> {"users":2,"tasks":0} [200]
  POST /tasks x4 (ada)                     -> 201, 201, 201, 429 (Retry-After: 30)
  POST /tasks {"title":""}                 -> 422 {"...","errors":{"title":["required"]}}
  ```

Not run here, but know them: **cookie auth** is `AddAuthentication().AddCookie()` + `Results.SignIn`; browser apps using cookies must also use `AddAntiforgery()`/`app.UseAntiforgery()` because browsers attach cookies automatically (CSRF). **API keys** are a custom header checked with `FixedTimeEquals` against stored hashes.

## 4. Common Pitfalls
| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `if (password == stored)` or `SequenceEqual` on secrets | `==` short-circuits on the first difference (timing leak) | `CryptographicOperations.FixedTimeEquals` on equal-length byte arrays |
| `ValidateIssuer = false` / `ValidateLifetime = false` "to make it work" | a 401 from a mismatched issuer/clock is annoying to debug | fix the issuer/audience config; tolerate drift with `ClockSkew`, never switch checks off |
| `ValidAlgorithms` not pinned | default accepts whatever the header says | `ValidAlgorithms = [SecurityAlgorithms.HmacSha256]` and reject `alg: none` |
| Short JWT secret or secret in `appsettings.json` | HS256 needs a >= 32-byte key; configs get committed | read from env/user-secrets/secret store; fail fast when too short |
| Expecting `ClaimTypes.Role` / `ClaimTypes.NameIdentifier` | the legacy `JwtSecurityTokenHandler` remapped `sub`/`role`; JsonWebToken does not (`MapInboundClaims = false`) | set `NameClaimType = "sub"`, `RoleClaimType = "role"` explicitly |
| `UseRateLimiter` before `UseAuthentication` | the partition key reads `HttpContext.User`, still anonymous | order: authentication, authorization, rate limiter |
| Fixed-window limiter allows 2x burst at the boundary | window resets at an instant; clients spend both windows back to back | token bucket or sliding window for user-facing limits |
| Building SQL with `$"... {input} ..."` | interpolated strings look harmless | `command.Parameters.AddWithValue("@x", v)` (or EF Core LINQ); never concatenate |
| Different errors for "no such user" vs "wrong password" | helpful messages leak which emails exist | one generic 401; hash a dummy password to equalise timing if needed |

## 5. Coming From Other Languages
- **Java/Spring Security:** filters + `SecurityFilterChain` become middleware order + `RequireAuthorization`; `@PreAuthorize("hasRole(...)")` becomes policies on the endpoint. `PasswordEncoder` ~ `PasswordHasher<T>`/`Pbkdf2`.
- **TypeScript/Express:** `passport-jwt` + `jsonwebtoken` ~ `AddJwtBearer` + `JsonWebTokenHandler`; `express-rate-limit` ~ `AddRateLimiter`; `helmet` has no one-package equivalent (set headers in middleware).
- **Go:** `crypto/subtle.ConstantTimeCompare` ~ `FixedTimeEquals`; `golang.org/x/time/rate` ~ the token-bucket limiter; middleware is `app.Use` instead of `http.Handler` wrapping.
- **Python:** decorators (`@login_required`) ~ attributes/`RequireAuthorization`; `hmac.compare_digest` ~ `FixedTimeEquals`.

## 6. Exercises
Starter code and tests live in [exercises/](exercises/); reference solutions in [solutions/](solutions/). Tests are offline, deterministic and use `FakeTimeProvider`.

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Password hasher | Easy | PBKDF2 record format, constant-time verify, rehash policy | `dotnet test modules/11-auth-and-security-basics/exercises --filter "FullyQualifiedName~.Ex1_"` |
| 2 | Token service | Medium | JWT issue/validate with pinned algorithm and an injected clock | `dotnet test modules/11-auth-and-security-basics/exercises --filter "FullyQualifiedName~.Ex2_"` |
| 3 | Token-bucket limiter | Stretch | per-key bucket, continuous refill, `Retry-After`, thread safety | `dotnet test modules/11-auth-and-security-basics/exercises --filter "FullyQualifiedName~.Ex3_"` |

**Exercise 1 - [Ex1_PasswordHasher.cs](exercises/Ex1_PasswordHasher.cs)**
- **Task:** implement `Hash`, `Verify` and `NeedsRehash` for `pbkdf2-sha256$iterations$salt$hash` records (16-byte salt, 32-byte hash).
- **Acceptance:** format test, salt uniqueness, wrong/empty password, six malformed records return `false`, verification uses the *stored* iteration count, rehash detection, null guard.
- <details><summary>Hint</summary>Pattern-match `stored.Split('$')` against a list pattern and use `Convert.TryFromBase64String` so garbage never throws.</details>

**Exercise 2 - [Ex2_TokenService.cs](exercises/Ex2_TokenService.cs)**
- **Task:** implement `Issue`/`Validate` (HS256, `sub` + `role`, issuer/audience, lifetime from `TokenOptions`); `Validate` returns `null` for anything invalid; the constructor rejects secrets shorter than 32 bytes.
- **Acceptance:** round trip, expiry driven by `FakeTimeProvider`, foreign secret/issuer/audience, tampered payload, garbage, `alg: none`, and an HS384 token signed with the right secret are all rejected.
- <details><summary>Hint</summary>`TokenValidationParameters.LifetimeValidator` lets you decide "now" with `time.GetUtcNow()`; `ValidAlgorithms` handles the algorithm tests.</details>

**Exercise 3 - [Ex3_TokenBucketLimiter.cs](exercises/Ex3_TokenBucketLimiter.cs)**
- **Task:** `TryAcquire(key)` returns `Allowed` and, when denied, the exact `RetryAfter` until one whole token exists. Buckets start full, refill continuously, cap at `burst`, are independent per key and safe under `Parallel.For`.
- **Acceptance:** burst then deny, fractional refill and wait time, slow rates (0.5/s), cap after a long idle, key isolation, exactly 10 of 200 concurrent calls pass, invalid settings throw.
- <details><summary>Hint</summary>Store `(tokens, lastTimestamp)` per key, refill with `time.GetElapsedTime`, and `lock` the single bucket while you read-modify-write it.</details>

## 7. Recap
- Store `pbkdf2-sha256$iterations$salt$hash`, verify with `FixedTimeEquals`, plan for rehashing.
- Validate JWTs completely (signature, algorithm, issuer, audience, lifetime) and keep them short-lived.
- Declare access rules on endpoints (`RequireAuthorization`, policies); order the middleware correctly.
- Parameterize SQL, encode output, use antiforgery whenever cookies authenticate browsers.
- Reach for the token bucket for per-user limits; return `429` with `Retry-After`.

## 8. Self-check Questions
1. Why does the PBKDF2 record include the iteration count, and what does that let you do in a year?
2. A token validates locally but is rejected in production: list three `TokenValidationParameters` that could explain it.
3. Why is a Bearer-token API not vulnerable to CSRF while a cookie-authenticated one is?
4. What happens to `ada`'s rate limit if `UseRateLimiter` runs before `UseAuthentication`?
5. Why return the same 401 for an unknown email and a wrong password?

## 9. Further Reading
- [Authentication and authorization in ASP.NET Core](https://learn.microsoft.com/aspnet/core/security/)
- [Rate limiting middleware](https://learn.microsoft.com/aspnet/core/performance/rate-limit)
- [OWASP Password Storage Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html) and [JWT Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/JSON_Web_Token_for_Java_Cheat_Sheet.html)

---
**Next:** [12 Testing](../12-testing/README.md)
