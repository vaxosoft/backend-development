# 11. Authentication and Security Basics in Rust

> Estimated time: 2h | Difficulty: Intermediate | Prerequisites: 02 (HTTP server), 03 (REST API design), 04 (serialization), 06 (concurrency basics: `spawn_blocking`)

## Learning Outcomes
After this module you can:
- [ ] Hash and verify passwords with Argon2id (random salt, PHC string, cost parameters) without blocking the async runtime
- [ ] Issue and validate JWTs with a pinned algorithm and strict expiry, and explain what a JWT does and does not protect
- [ ] Store API keys as keyed hashes and compare secrets in constant time
- [ ] Protect routes with axum middleware (authentication, then role-based authorization) and return a consistent 401/403 envelope
- [ ] Implement a token-bucket rate limiter and answer `429` with `Retry-After`
- [ ] Explain the basics of sessions/cookies, SQL injection, XSS, CSRF and TLS, and which ones Rust's type system does (not) help with

## 1. Concept Overview
**Authentication** answers "who are you?" (password, token, API key); **authorization** answers "may you do this?" (roles, ownership). Always in that order: 401 means "I don't know who you are", 403 means "I know, and the answer is no".

| Topic | Rule of thumb |
|-------|---------------|
| Passwords | Never store or log them. Store a salted, deliberately slow hash (Argon2id). |
| Sessions / cookies | Server remembers state, browser holds an opaque id: `HttpOnly; Secure; SameSite=Lax`. |
| JWT | Signed (not encrypted) claims; stateless, hard to revoke, so keep `exp` short. |
| API keys | Long random secrets for machines; store only a hash, show the key once. |
| Input handling | Validate at the edge; **bind** SQL parameters; **escape** on output (XSS). |
| CSRF | Only matters when the browser sends credentials automatically (cookies). Bearer headers are immune. |
| TLS | Terminate HTTPS at a reverse proxy/load balancer or with `rustls`; never roll your own crypto. |
| Rate limiting | Token bucket per user/IP: allows bursts, bounds the average rate. |

## 2. How Rust Approaches It
- **Crates (from the pinned set):** `argon2` (RustCrypto, PHC strings), `jsonwebtoken`, `hmac` + `sha2` (keyed hashes), `rand` (`rand::rng()` is a CSPRNG), `axum-extra` (`CookieJar`), `base64`, `hex`. The rate limiter is hand-written (20 lines).
- **Types carry the safety.** A `Claims` struct plus `Role` enum means a handler that takes `Extension<Claims>` cannot run without the middleware having verified the token, and `match` on `Role` is exhaustive.
- **Memory safety is not application security.** Rust removes buffer overflows, not logic bugs: SQL injection, missing authorization checks and weak JWT settings are all still possible.
- **Middleware = `axum::middleware::from_fn[_with_state]`**: an `async fn(State, Request, Next)` that returns `Result<Response, ApiError>`; `ApiError: IntoResponse` produces the envelope.
- **Constant-time compare:** `hmac::Mac::verify_slice` (never `==` on secrets).
- **No crypto backend in `jsonwebtoken` 11 by default.** It must be given `rust_crypto` or `aws_lc_rs` (Cargo features). This workspace pins default features, so [`src/lib.rs`](./src/lib.rs) installs a small HS256 `CryptoProvider` built on `hmac`+`sha2`. In your own project enable one feature and skip that file; the examples call `install_jwt_crypto()` once at start-up.

## 3. Guided Examples
Run all commands from `languages/rust/`.

### 3.1 Password hashing
- **Goal:** Argon2id with a random salt, verify, and `spawn_blocking`.
- **Run:** `cargo run -q -p m11-auth-and-security-basics --example password_hashing`
- **Code:** [examples/password_hashing.rs](./examples/password_hashing.rs)
- **Walkthrough:** `Argon2::default().hash_password(bytes)` picks a new random salt and returns a PHC string `$argon2id$v=19$m=19456,t=2,p=1$<salt>$<hash>`. Salt and cost live inside the string, so `verify_password` needs only the stored string. A malformed string is `false`, not a panic. Hashing takes tens of milliseconds and 19 MiB, so handlers run it on `tokio::task::spawn_blocking`.
  ```rust
  let stored = tokio::task::spawn_blocking(|| hash_password("correct horse battery staple")).await??;
  ```
- **Expected output:**
  ```
  phc prefix: $argon2id$v=19$m=19456,t=2,p=1
  contains plaintext: false
  same password, same hash: false
  right password: true
  wrong password: false
  malformed hash: false
  ```

### 3.2 JWT issue and validate
- **Goal:** sign HS256 tokens, validate with a pinned algorithm and strict expiry.
- **Run:** `cargo run -q -p m11-auth-and-security-basics --example jwt_tokens`
- **Code:** [examples/jwt_tokens.rs](./examples/jwt_tokens.rs)
- **Walkthrough:** `Validation::new(Algorithm::HS256)` rejects tokens whose header names another algorithm; `leeway = 0` and `set_required_spec_claims(&["exp","sub"])` make expiry mandatory and exact. A member who rewrites the payload to `admin` breaks the signature. The last line shows the payload is only base64: **never put secrets in claims**.
  ```rust
  let mut v = Validation::new(Algorithm::HS256);
  v.leeway = 0;
  v.set_required_spec_claims(&["exp", "sub"]);
  ```
- **Expected output:**
  ```
  three segments: true
  valid token:    ok sub=7 role=member
  expired token:  rejected: expired
  wrong secret:   rejected: bad signature
  tampered:       rejected: bad signature
  header HS512:   rejected: wrong algorithm
  payload (readable by anyone): {"sub":"7","role":"member","exp":4102444800}
  ```

### 3.3 API keys and constant-time comparison
- **Goal:** random key shown once, only an HMAC digest stored, verified in constant time.
- **Run:** `cargo run -q -p m11-auth-and-security-basics --example api_keys`
- **Code:** [examples/api_keys.rs](./examples/api_keys.rs)
- **Walkthrough:** the key is `tt_<id>_<secret>`; the id is a lookup handle, the 32-byte secret is random so a fast keyed hash (HMAC-SHA256 with a server-side pepper) is enough: Argon2 is for low-entropy human passwords. `Mac::verify_slice` compares in constant time.
  ```rust
  self.mac(secret).verify_slice(&stored.digest).ok()?;
  ```
- **Expected output** (the key itself is random; lengths are not):
  ```
  key length: 55
  key prefix: tt_
  authenticate(valid key):   Some(42)
  authenticate(altered key): None
  authenticate(garbage):     None
  two keys differ: true
  second key owner: Some(43)
  ```

### 3.4 Token-bucket rate limiter
- **Goal:** burst + steady rate, with an injected clock so behaviour is testable.
- **Run:** `cargo run -q -p m11-auth-and-security-basics --example token_bucket`
- **Code:** [examples/token_bucket.rs](./examples/token_bucket.rs)
- **Walkthrough:** `try_take(now: Instant)` refills `elapsed * rate` tokens (capped at capacity), then takes one or returns how long until one exists. Taking `now` as an argument lets tests and this demo advance time without sleeping. Capacity 3, one token per 2 s:
- **Expected output:**
  ```
  t=    0ms: allowed
  t=    0ms: allowed
  t=    0ms: allowed
  t=    0ms: denied, Retry-After: 2
  t= 1000ms: denied, Retry-After: 1
  t= 2000ms: allowed
  t= 2100ms: denied, Retry-After: 2
  t=10000ms: allowed
  t=10000ms: allowed
  t=10000ms: allowed
  t=10000ms: denied, Retry-After: 2
  ```

### 3.5 Task Tracker with auth (server)
- **Goal:** register, login, bearer middleware, admin-only route and a per-user rate limit on `POST /tasks`.
- **Run:** `ADDR=127.0.0.1:20700 cargo run -q -p m11-auth-and-security-basics --example tracker_auth` (Ctrl-C or SIGTERM stops it gracefully; `JWT_SECRET` overrides the demo secret)
- **Code:** [examples/tracker_auth.rs](./examples/tracker_auth.rs)
- **Walkthrough:**
  - `register` validates, hashes on `spawn_blocking`, and makes the first account `admin` (demo rule).
  - `login` returns the **same** 401 for unknown email and wrong password, and verifies against a dummy hash when the user is unknown so timing does not reveal which emails exist.
  - `require_auth` (`from_fn_with_state`) parses `Authorization: Bearer`, validates the JWT and inserts `Claims` into request extensions; `require_admin` reads them with `Extension<Claims>`. With `route_layer`, the later (outer) layer runs first, so authentication runs before the role check.
  - `create_task` takes a token from a per-user bucket (burst 2, 0.2/s) and answers `429` + `Retry-After`.
- **Try it:**
  ```bash
  B=http://127.0.0.1:20700; J='Content-Type: application/json'
  curl -s -XPOST $B/auth/register -H "$J" -d '{"email":"ada@example.com","password":"short"}'
  curl -s -XPOST $B/auth/register -H "$J" -d '{"email":"ada@example.com","password":"correct-horse"}'
  curl -s -XPOST $B/auth/register -H "$J" -d '{"email":"bob@example.com","password":"battery-staple"}'
  curl -si -XPOST $B/auth/login -H "$J" -d '{"email":"nobody@example.com","password":"wrong-password"}'
  BOB=$(curl -s -XPOST $B/auth/login -H "$J" -d '{"email":"bob@example.com","password":"battery-staple"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["token"])')
  curl -si $B/tasks
  for i in 1 2 3; do curl -si -XPOST $B/tasks -H "$J" -H "Authorization: Bearer $BOB" -d "{\"title\":\"Task $i\"}"; done
  curl -si $B/admin/users -H "Authorization: Bearer $BOB"
  ```
- **Expected output** (headers trimmed to the relevant ones):
  ```
  422  {"error":{"code":"validation_failed","message":"password must be at least 8 characters"}}
  201  {"email":"ada@example.com","id":1,"role":"admin"}
  201  {"email":"bob@example.com","id":2,"role":"member"}
  401  {"error":{"code":"unauthorized","message":"invalid email or password"}}
  401  www-authenticate: Bearer    {"error":{"code":"unauthorized","message":"missing or invalid bearer token"}}
  201  {"id":1,"owner_id":2,"title":"Task 1"}
  201  {"id":2,"owner_id":2,"title":"Task 2"}
  429  retry-after: 5    {"error":{"code":"rate_limited","message":"too many requests"}}
  403  {"error":{"code":"forbidden","message":"requires the admin role"}}
  ```

### Sessions/cookies, XSS, CSRF, TLS in one paragraph each
- **Cookies:** `axum_extra::extract::cookie::{CookieJar, Cookie}`: `Cookie::build(("sid", id)).http_only(true).secure(true).same_site(SameSite::Lax).path("/")`. Keep the session data server-side; the cookie is only a random id.
- **SQL injection:** `sqlx::query("... WHERE email = ?").bind(email)`: values never become SQL text. Never `format!` user input into a query (module 05).
- **XSS:** a JSON API is safe by default (`Content-Type: application/json`); when you generate HTML, escape on output (`minijinja` autoescaping, module 13) and send `X-Content-Type-Options: nosniff`.
- **CSRF:** with cookie auth use `SameSite=Lax/Strict` plus a CSRF token (double-submit) on state-changing requests; bearer-token APIs don't need it.
- **TLS:** run behind a TLS-terminating proxy, or use `rustls`; set HSTS at the proxy. Locally, plain HTTP on loopback is fine.

## 4. Common Pitfalls
| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| Hashing with Argon2 directly in an `async fn` handler | It is CPU+memory heavy; it stalls the runtime worker and every task on it | `tokio::task::spawn_blocking` (or a dedicated pool) |
| `jsonwebtoken` panics: "Could not automatically determine the process-level CryptoProvider" | v11 has no crypto backend unless a feature is on | Enable `rust_crypto` or `aws_lc_rs` (or install a provider, as `install_jwt_crypto()` does) before the first `encode`/`decode` |
| `Validation::default()` (HS256, 60 s leeway) trusted blindly | Default leeway accepts "expired" tokens; algorithm may not match your key family | Build `Validation::new(Algorithm::HS256)`, set `leeway = 0`, require `exp` |
| `if token == stored` / `a == b` on secrets | `==` on `&[u8]` returns at the first differing byte (timing side channel) | `Mac::verify_slice`, or a constant-time compare |
| Different errors for "no such user" and "wrong password" | Convenient error mapping with `?` | One `ApiError::BadCredentials`; verify a dummy hash for unknown users |
| `.layer()` ordering confusion: role check runs before auth | In axum, the layer added last is outermost and runs first | Add `require_auth` after `require_admin`; test 401 vs 403 |
| `layer` after `route` does not cover routes added later | `route_layer`/`layer` only wrap routes already registered | Register routes first, then the middleware, as in `app()` of exercise 3 |
| Logging the `Authorization` header or the password struct with `{:?}` | `#[derive(Debug)]` prints every field | Skip secrets in `tracing` fields; wrap in `secrecy::SecretString` (module 09) |
| `std::sync::Mutex` guard held across `.await` in the limiter | Natural when the check and the work share a function | Lock, update the bucket, drop the guard, then `.await` |

## 5. Coming From Other Languages
- **Go:** middleware is `func(http.Handler) http.Handler`; in axum it is an async fn with `Next`. `subtle.ConstantTimeCompare` ↔ `Mac::verify_slice`. Context values ↔ request `Extension`s, but typed and checked at compile time.
- **Java/C#:** no `[Authorize]` attribute or filter pipeline; attach layers to a `Router` (or sub-router). Policies become plain `match` on a `Role` enum.
- **Python/TypeScript:** no decorators; a failed extractor/middleware returns `Err(ApiError)` which short-circuits the handler, much like an exception filter but visible in the type.
- **C++:** RAII-style zeroisation is available via `zeroize`/`secrecy`; the compiler will not stop you from logging secrets, though.

## 6. Exercises
Starters compile and fail on `todo!()`; solutions are in [solutions/](./solutions).

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Password service | Easy | Argon2id, PHC strings, parameter upgrade, Unicode-aware policy | `cargo test -p m11-ex1-password-hasher` |
| 2 | Keyed token-bucket limiter | Medium | Injected clock, refill math, `Retry-After`, idle eviction | `cargo test -p m11-ex2-token-bucket` |
| 3 | Bearer middleware and roles | Stretch | JWT validation settings, `from_fn_with_state`, 401 vs 403 | `cargo test -p m11-ex3-bearer-middleware` |

**Exercise 1: [exercises/ex1-password-hasher](./exercises/ex1-password-hasher)**
- **Task:** implement `check_policy`, `PasswordService::{hash, verify, needs_rehash}`.
- **Acceptance:** PHC `$argon2id$` output without plaintext; fresh salt each time; wrong password is `Ok(false)`, malformed hash is `Err(Malformed)`; old hashes still verify; `needs_rehash` compares memory/time cost; the policy counts characters, not bytes.
- <details><summary>Hint</summary>`Params::try_from(&PasswordHash)` reads the parameters back out of a stored hash.</details>

**Exercise 2: [exercises/ex2-token-bucket](./exercises/ex2-token-bucket)**
- **Task:** implement `RateLimiter::check`, `evict_idle` and `Denied::retry_after_secs`.
- **Acceptance:** burst then deny; keys are independent; refill is proportional and capped; `retry_after` is exact; clocks that go backwards mint no tokens; only full-again buckets are evicted.
- <details><summary>Hint</summary>Use `saturating_duration_since`, and keep tokens as `f64`.</details>

**Exercise 3: [exercises/ex3-bearer-middleware](./exercises/ex3-bearer-middleware)**
- **Task:** implement `Auth::verify`, `require_auth` and `require_role` (token issuing and the router are given).
- **Acceptance:** the same 401 + `WWW-Authenticate: Bearer` for missing, malformed, expired, foreign-signed, `alg: none`, HS512 and `exp`-less tokens; 403 for a member on an admin route; case-insensitive scheme.
- <details><summary>Hint</summary>A single `Validation` (pinned `Algorithm::HS256`, `leeway = 0`, required `exp`) covers most of the 401 cases; collapse all failures into one error value.</details>

## 7. Recap
- Hash passwords with Argon2id on a blocking thread; the PHC string carries salt and cost.
- JWT = signed, readable claims: pin the algorithm, require a short `exp`, keep secrets out.
- Authenticate first (401), authorize second (403); one generic failure message for credentials.
- Compare secrets in constant time; bind SQL parameters; escape HTML on output.
- Reach for a token bucket when you need bursts plus a bound on the average rate.

## 8. Self-check Questions
1. Why does `verify_password` need no separate salt or parameter arguments, and what does `needs_rehash` let you do at login time?
2. A teammate adds `.layer(require_admin)` and then `.layer(require_auth)` to a router. Which one runs first, and what status does an anonymous caller get?
3. Why is HMAC-SHA256 acceptable for API keys but not for user passwords?
4. Your API authenticates only with `Authorization` headers. Why is CSRF not a concern, and what changes if you move the token into a cookie?

## 9. Further Reading
- OWASP Cheat Sheets: Password Storage, JSON Web Token, Cross-Site Request Forgery Prevention
- [`argon2`](https://docs.rs/argon2), [`jsonwebtoken`](https://docs.rs/jsonwebtoken), [axum middleware guide](https://docs.rs/axum/latest/axum/middleware/index.html)
- RFC 7519 (JWT), RFC 6750 (Bearer tokens), RFC 6585 (429)

---
**Next:** [12 Testing](../12-testing/README.md)
