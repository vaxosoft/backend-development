# 11. Authentication and Security Basics in TypeScript

> Estimated time: 2h | Difficulty: Intermediate | Prerequisites: 02 (Fastify), 03 (error envelope, zod), 05 (SQL basics), 10 (logging)

## Learning Outcomes

After this module you can:

- [ ] Hash and verify passwords with `scrypt`, a random salt and a constant-time comparison.
- [ ] Issue and verify JWTs with `jose`, pinning the algorithm and checking `exp` and the issuer.
- [ ] Write bearer-token authentication and role-based authorization as Fastify hooks.
- [ ] Prevent SQL injection, XSS and CSRF, and set safe cookie flags.
- [ ] Apply a per-user token-bucket rate limit and know where TLS fits.

## 1. Concept Overview

**Authentication** answers "who are you?" (password, token, API key); **authorization** answers "what may you do?" (roles, ownership). The server must never store passwords, only slow salted hashes; must never trust the client for identity beyond a signed, expiring credential; and must treat _all_ input as hostile: SQL goes through parameters, output is encoded for its context, state-changing browser requests need a CSRF defence.

```text
login: password --scrypt(salt)--> compare in constant time --> signed token (sub, role, exp)
request: Authorization: Bearer <token> --> verify (alg, signature, exp, iss) --> req.user --> role check --> handler
```

Rate limiting caps what one caller can do per unit of time (a **token bucket** allows a burst, then a steady rate). TLS (HTTPS) protects credentials in transit; in production it usually terminates at a reverse proxy.

## 2. How TypeScript Approaches It

- **`node:crypto`**: `scrypt` (memory-hard KDF, built in, no native addon), `randomBytes`, `timingSafeEqual`, `createHmac`. No bcrypt/argon2 dependency needed.
- **`jose`**: dependency-free JWT/JWS on Web Crypto. `SignJWT` builds tokens, `jwtVerify` checks signature and claims; typed `errors.*` classes with stable `code`s.
- **Fastify hooks**: `preHandler` functions as middleware; `decorateRequest` for `req.user` (typed through `declare module 'fastify'`).
- **`@fastify/cookie`**: signed cookies and flags. **`better-sqlite3`** prepared statements for parameterised SQL. **`zod`** validates untrusted input (TypeScript types are erased at runtime and prove nothing about a request body).
- Idioms: functions return `boolean`/discriminated unions for expected failures (`verifyPassword` -> `false`), and the HTTP layer turns them into 401/403. Inject the clock (`now`) so expiry and rate limits are testable.

## 3. Guided Examples

Run all commands from `languages/typescript/`. Shared helpers (`hashPassword`, `verifyPassword`, `TokenBucketLimiter`) are in [examples/_shared.ts](examples/_shared.ts).

### 3.1 Password hashing with scrypt

- **Goal:** store `scrypt$cost$salt$hash`, verify in constant time.
- **Run:** `node modules/11-auth-and-security-basics/examples/01_password_hashing.ts`
- **Code:** [examples/01_password_hashing.ts](examples/01_password_hashing.ts)

```ts
const salt = randomBytes(16);
const key = await derive(password, salt, cost);        // scrypt(password, salt, 32, { N: cost })
return `scrypt$${cost}$${salt.toString('base64url')}$${key.toString('base64url')}`;
// verify:
return expected.length === actual.length && timingSafeEqual(expected, actual);
```

- **Walkthrough:** a fresh random salt per password defeats rainbow tables and makes equal passwords hash differently. The cost is stored in the string so it can be raised later. `timingSafeEqual` compares without leaking _where_ the first difference is, but throws on different lengths, so the length is checked first. Malformed stored values simply return `false`.
- **Expected output:**

```text
format: scrypt 16384 salt=22 chars hash=43 chars
right password: true
wrong password: false
malformed hash: false
same password, different hashes: true
both verify: true
timingSafeEqual on unequal lengths throws: ERR_CRYPTO_TIMING_SAFE_EQUAL_LENGTH
```

### 3.2 JWTs with `jose`

- **Goal:** sign, verify, and see exactly which attacks are rejected.
- **Run:** `node modules/11-auth-and-security-basics/examples/02_jwt.ts`
- **Code:** [examples/02_jwt.ts](examples/02_jwt.ts)

```ts
const { payload } = await jwtVerify(jwt, secret, { algorithms: ['HS256'], issuer: 'task-tracker' });
```

- **Walkthrough:** the token is signed once, then verified against a fixed clock (`currentDate`) so results are repeatable. `algorithms: ['HS256']` is the pin that rejects `alg: none` and algorithm-confusion tricks; `jwtVerify` always enforces `exp`. A JWT is signed, **not encrypted**: anyone can read the payload, so keep secrets out of it.
- **Expected output:**

```text
parts: 3
valid: ok sub=42 role=member alg=HS256
expired: rejected ERR_JWT_EXPIRED
tampered payload: rejected ERR_JWS_SIGNATURE_VERIFICATION_FAILED
wrong secret: rejected ERR_JWS_SIGNATURE_VERIFICATION_FAILED
alg none: rejected ERR_JOSE_ALG_NOT_ALLOWED
```

### 3.3 SQL injection and XSS

- **Goal:** why parameters and output encoding are non-negotiable.
- **Run:** `node modules/11-auth-and-security-basics/examples/03_injection_and_xss.ts`
- **Code:** [examples/03_injection_and_xss.ts](examples/03_injection_and_xss.ts)
- **Walkthrough:** `WHERE owner_id = ${hostile}` turns `1 OR 1=1` into SQL and leaks every row; `prepare('... = ?').all(value)` keeps it data. Identifiers (`ORDER BY`) cannot be parameters, so they come from an allow-list. For XSS the title is HTML-encoded before it goes into markup (a template engine or framework does this by default; raw string concatenation does not).
- **Expected output:**

```text
concatenated SQL leaks: 2 rows
bound parameter returns: 0 rows
owner 1 sees: [ { title: 'alice: pay rent' } ]
sort=newest: [ { title: 'bob: secret plan' }, { title: 'alice: pay rent' } ]
rejected: cannot sort by "title; DROP TABLE tasks"
raw : <li><img src=x onerror="alert(1)"></li>
safe: <li>&lt;img src=x onerror=&quot;alert(1)&quot;&gt;</li>
```

### 3.4 Cookies and CSRF

- **Goal:** session cookie flags and a CSRF token check.
- **Run:** `node modules/11-auth-and-security-basics/examples/04_cookies_and_csrf.ts`
- **Code:** [examples/04_cookies_and_csrf.ts](examples/04_cookies_and_csrf.ts)

```ts
reply.setCookie('sid', sessionId, {
  httpOnly: true,
  secure: true,
  sameSite: 'lax',
  path: '/',
  maxAge: 3600,
  signed: true,
});
```

- **Walkthrough:** `HttpOnly` hides the cookie from scripts, `Secure` limits it to HTTPS, `SameSite=Lax` stops browsers attaching it to cross-site POSTs, signing detects tampering. Because cookies are sent _automatically_, state-changing routes also require a token the attacker's page cannot know: here an HMAC of the session id sent in `X-CSRF-Token`. Bearer tokens in the `Authorization` header are not sent automatically, so pure token APIs are not CSRF-prone (but should not live in `localStorage` in a browser app).
- **Expected output:**

```text
cookie attributes: Max-Age=3600 | Path=/ | HttpOnly | Secure | SameSite=Lax
no cookie: 401 {"error":{"code":"unauthorized","message":"login required"}}
cookie only (forged cross-site form): 403 {"error":{"code":"forbidden","message":"csrf token missing or wrong"}}
cookie + wrong token: 403 {"error":{"code":"forbidden","message":"csrf token missing or wrong"}}
cookie + token: 201 {"created":true}
tampered cookie: 401 {"error":{"code":"unauthorized","message":"login required"}}
```

### 3.5 Task Tracker API: login, JWT, roles, rate limit

- **Goal:** the pieces combined in a service. `ADDR` defaults to `127.0.0.1:8080`; Ctrl-C/SIGTERM closes it. Users: `alice@example.com` / `alice-pass-1` (admin) and `bob@example.com` / `bob-pass-12` (member). Set `JWT_SECRET` (32+ characters) to keep tokens valid across restarts; without it a random secret is generated per run.
- **Run:** `ADDR=127.0.0.1:22351 node modules/11-auth-and-security-basics/examples/05_task_tracker_auth.ts`
- **Code:** [examples/05_task_tracker_auth.ts](examples/05_task_tracker_auth.ts)

```ts
const verdict = limiter.take(`user:${req.user?.id}`);          // TokenBucketLimiter({ burst: 3, refillPerSecond: 1 })
if (!verdict.allowed) return reply.code(429).header('retry-after', String(verdict.retryAfterSeconds)).send(...);
app.delete('/tasks/:id', { preHandler: [authenticate, requireRole('admin')] }, handler);
```

- **Walkthrough:** unknown user and wrong password produce the same 401 (and the same hashing work, via a decoy hash), so the endpoint does not reveal which emails exist. `authenticate` verifies the bearer token with pinned algorithm and issuer. The limiter is keyed by user id; the bucket holds 3 tokens and refills 1 per second. In another terminal:

```bash
B=http://127.0.0.1:22351; J='content-type: application/json'
login() { curl -s -X POST $B/auth/login -H "$J" -d "$1"; }
curl -s -w '\n%{http_code}\n' $B/tasks
login '{"email":"alice@example.com","password":"nope"}'; echo
login '{"email":"ghost@example.com","password":"nope"}'; echo
ALICE=$(login '{"email":"alice@example.com","password":"alice-pass-1"}' | sed -E 's/.*"token":"([^"]+)".*/\1/')
BOB=$(login '{"email":"bob@example.com","password":"bob-pass-12"}' | sed -E 's/.*"token":"([^"]+)".*/\1/')
for i in 1 2 3 4; do curl -s -w ' %{http_code} retry-after=%header{retry-after}\n' -X POST $B/tasks -H "authorization: Bearer $BOB" -H "$J" -d '{"title":"Task '$i'"}'; done
curl -s -w '\n%{http_code}\n' -X DELETE $B/tasks/1 -H "authorization: Bearer $BOB"
curl -s -w '%{http_code}\n' -X DELETE $B/tasks/1 -H "authorization: Bearer $ALICE"
curl -s -w '%{http_code} %header{www-authenticate}\n' $B/tasks -H "authorization: Bearer ${ALICE}x"
```

- **Expected output:**

```text
{"error":{"code":"unauthorized","message":"missing or invalid token"}}
401
{"error":{"code":"unauthorized","message":"invalid credentials"}}
{"error":{"code":"unauthorized","message":"invalid credentials"}}
{"id":1,"title":"Task 1","owner_id":2} 201 retry-after=
{"id":2,"title":"Task 2","owner_id":2} 201 retry-after=
{"id":3,"title":"Task 3","owner_id":2} 201 retry-after=
{"error":{"code":"rate_limited","message":"too many requests"}} 429 retry-after=1
{"error":{"code":"forbidden","message":"requires role admin"}}
403
204
{"error":{"code":"unauthorized","message":"missing or invalid token"}}401 Bearer
```

### TLS in one paragraph

Credentials must never cross the network in clear text. Either terminate TLS at a reverse proxy/load balancer (the usual production setup) or let Node do it: `Fastify({ https: { key, cert, minVersion: 'TLSv1.2' } })` (it uses `node:https` underneath). Keep keys out of the repository, load them from files or a secret store, and never disable certificate verification (`rejectUnauthorized: false`) in clients to "make it work".

## 4. Common Pitfalls

| Mistake                                                              | Why it happens                                                              | Fix                                                                                              |
| -------------------------------------------------------------------- | --------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ |
| `token === expected` or `hash === stored` on secrets                 | `===` is the obvious comparison, and it returns at the first differing byte | `timingSafeEqual` on equal-length `Buffer`s (check lengths first: it throws otherwise)           |
| `jwtVerify(token, secret)` without `algorithms`                      | The default accepts every algorithm valid for the key type                  | Always pass `algorithms: ['HS256']`, plus `issuer`/`audience`                                    |
| Using `decodeJwt` (or `JSON.parse` of the payload) as verification   | Decoding is easy and "works"                                                | Only `jwtVerify` proves anything; decoded payloads are untrusted                                 |
| Hard-coded or short JWT secret in source                             | Copy-paste from tutorials                                                   | Read `JWT_SECRET` from config, require 32+ bytes, fail fast at startup                           |
| Fast hashes (`sha256`, `md5`) for passwords, or a constant salt      | Hashing "just works"                                                        | Use `scrypt` with `randomBytes(16)` per password and store the cost                              |
| Blocking the event loop with `scryptSync`                            | The sync API is shorter                                                     | Use the callback/promise `scrypt`, which runs on the libuv thread pool                           |
| `localStorage` for tokens in a browser app                           | Simple to read from JavaScript                                              | An XSS bug steals it: prefer `HttpOnly; Secure; SameSite` cookies (and then add CSRF protection) |
| Casting the body (`req.body as Login`) instead of validating         | TypeScript types are erased at runtime                                      | `zod` `safeParse` every body, query and param                                                    |
| `req.user` never set, or optional chaining hides a missing auth hook | `req.user?.id` compiles even if no hook ran                                 | Make the hook part of the route (`preHandler`), and test the 401 path                            |
| Template-string SQL (`` `... ${id}` ``)                              | Looks like harmless interpolation                                           | `db.prepare('... ?').all(id)`; allow-list identifiers                                            |
| Different errors for "unknown user" and "wrong password"             | Helpful messages                                                            | One message, and run the hash for both                                                           |

## 5. Coming From Other Languages

- **JavaScript/Node:** `crypto.scrypt` replaces the usual `bcrypt` npm package, avoiding native builds.
- **Java/C#:** Spring Security / ASP.NET Identity ship filters and a user store; Fastify gives you hooks and you compose the pieces yourself. `jose` plays the role of `System.IdentityModel.Tokens.Jwt` / nimbus-jose.
- **Python:** same ideas as FastAPI dependencies (`Depends(get_current_user)`); `preHandler` is the equivalent.
- **Go:** middleware wrapping `http.Handler` is the same shape as Fastify `preHandler`; `subtle.ConstantTimeCompare` is `timingSafeEqual` (without the length throw).

## 6. Exercises

Each exercise has starter code and tests in [exercises/](exercises/); solutions are in [solutions/](solutions/).

| #   | Title                                                         | Difficulty | Skill practiced                                     | Run tests                                                                                 |
| --- | ------------------------------------------------------------- | ---------- | --------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| 1   | [Password hashing](exercises/ex1_password/password.ts)        | Easy       | scrypt, salts, constant-time compare, rehash policy | `node --test "modules/11-auth-and-security-basics/exercises/ex1_password/*.test.ts"`      |
| 2   | [Token service](exercises/ex2_token_service/token-service.ts) | Medium     | `jose`, pinned algorithm, expiry, claim validation  | `node --test "modules/11-auth-and-security-basics/exercises/ex2_token_service/*.test.ts"` |
| 3   | [Auth guard](exercises/ex3_auth_guard/auth-guard.ts)          | Stretch    | bearer auth, roles, rate limit, security headers    | `node --test "modules/11-auth-and-security-basics/exercises/ex3_auth_guard/*.test.ts"`    |

**Exercise 1 - `hashPassword`, `verifyPassword`, `needsRehash`**

- **Task:** implement the `scrypt$cost$salt$hash` format; verification must never throw on bad input and must compare in constant time; `needsRehash` supports raising the cost later.
- **Acceptance:** all tests in `password.test.ts` pass (they use a low cost to stay fast).
- <details><summary>Hint</summary>Parse first and return `false` for anything unexpected; compare lengths before `timingSafeEqual`; wrap the derivation in `try/catch` for invalid parameters.</details>

**Exercise 2 - `createTokenService`**

- **Task:** an HS256 token service whose `verify` returns `{ ok: true, claims }` or `{ ok: false, reason: 'expired' | 'invalid' }` and never throws.
- **Acceptance:** expiry follows the injected clock; tampered, `alg: none`, HS512, wrong-issuer and bad-claim tokens are `invalid`.
- <details><summary>Hint</summary>`jwtVerify` options `algorithms`, `issuer`, `currentDate`, `requiredClaims`; `errors.JWTExpired` identifies expiry. Validate `sub` and `role` yourself afterwards.</details>

**Exercise 3 - `buildApp` with authentication, roles and rate limiting**

- **Task:** build the secured API described in the file's doc comment using the injected `verifyToken` and `limiter`.
- **Acceptance:** 401 vs 403 vs 429 behave as specified; anonymous requests cost no rate-limit budget; headers are set on every response.
- <details><summary>Hint</summary>One `authenticate` preHandler that sets `req.claims`; an `onSend` hook for `nosniff`; order preHandlers `[authenticate, requireAdmin]`.</details>

## 7. Recap

- Passwords: scrypt + random salt + constant-time compare; store the cost.
- Tokens: verify signature, pin the algorithm, check `exp` and issuer; a JWT is signed, not secret.
- Authentication first (401), then authorization (403); same error for unknown user and wrong password.
- Parameterise SQL, encode output, flag cookies `HttpOnly; Secure; SameSite`, add CSRF tokens for cookie sessions, rate-limit per user, use TLS.
- Reach for `node:crypto` and `jose` before any third-party auth package.

## 8. Self-check Questions

1. Why can `timingSafeEqual` throw, and how do you avoid turning that into a 500?
2. What attack does `algorithms: ['HS256']` prevent, and what happens without it?
3. Why do cookie sessions need CSRF protection but `Authorization: Bearer` APIs usually do not?
4. A token bucket has burst 3 and refill 1/s. How many requests succeed in the first second, and what `Retry-After` does the fourth get?
5. Why store the scrypt cost inside the hash string?

## 9. Further Reading

- OWASP Cheat Sheets: Password Storage, JSON Web Token, CSRF Prevention, SQL Injection Prevention
- `jose` README: [github.com/panva/jose](https://github.com/panva/jose)
- Node.js `crypto` documentation (`scrypt`, `timingSafeEqual`)

---

**Next:** [12 Testing](../12-testing/README.md)
