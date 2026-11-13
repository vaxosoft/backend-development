# 11. Authentication and Security Basics in Python

> Estimated time: 2 h | Difficulty: Intermediate | Prerequisites: [02 HTTP server](../02-http-server/README.md), [03 REST API design](../03-rest-api-design/README.md), [05 Database access](../05-database-access/README.md), [10 Logging](../10-logging-and-diagnostics/README.md)

## Learning Outcomes

After this module you can:

- [ ] Hash and verify passwords with salted, slow `scrypt` and compare secrets in constant time
- [ ] Issue and verify JWTs correctly (pinned algorithm, expiry, required claims)
- [ ] Protect FastAPI routes with `Depends`-based authentication and role checks
- [ ] Generate tokens and API keys with `secrets` and store only their digests
- [ ] Prevent SQL injection and XSS and explain what CSRF is and when it applies
- [ ] Add a token-bucket rate limit that answers `429` with `Retry-After`
- [ ] Explain what TLS gives you and where it is terminated

## 1. Concept Overview

**Authentication** answers "who are you?" (password, token, API key); **authorization** answers "what may you do?" (roles, ownership). They fail differently: no or bad credentials is `401`, valid credentials without permission is `403`.

```text
POST /auth/login  (email + password) -> verify scrypt hash -> JWT (sub, role, exp)
GET  /tasks/1     Authorization: Bearer <JWT> -> verify signature + exp -> principal -> role check -> handler
```

Rules that hold in every language:

- Never store passwords: store a **slow, salted hash** and compare in constant time. Never invent your own scheme.
- Tokens must **expire**, be verified with a **pinned algorithm** and carry only non-secret claims (a JWT is signed, not encrypted).
- Treat all input as hostile: validate its shape, bind it as a **parameter** in SQL, and **escape** it for the context it is printed in.
- **CSRF** tricks a browser into sending a cookie-authenticated request from another site. Bearer tokens in an `Authorization` header are not sent automatically, so they are not CSRF-prone; cookie sessions need `SameSite`, `HttpOnly`, `Secure` and a CSRF token.
- **TLS** (HTTPS) encrypts and authenticates the connection. Most deployments terminate TLS at a reverse proxy or load balancer; the app then trusts only that hop.
- **Rate limiting** bounds the damage of password guessing and abuse.

## 2. How Python Approaches It

| Need | Tool | Note |
|------|------|------|
| Password hash | `hashlib.scrypt` | memory-hard KDF in the stdlib; parameters `n`, `r`, `p` stored with the hash |
| Random secrets | `secrets` (`token_bytes`, `token_urlsafe`) | **never** `random`: it is predictable |
| Constant-time compare | `hmac.compare_digest` | never `==` on secrets or MACs |
| JWT | `PyJWT` (`import jwt`) | `jwt.encode` / `jwt.decode(..., algorithms=[...])`; errors derive from `jwt.InvalidTokenError` |
| Auth in handlers | FastAPI `Depends`, `HTTPBearer`, `APIKeyHeader` | a dependency is a function; it can raise, and routes list it in their signature |
| SQL safety | `?` placeholders in `sqlite3` (or SQLAlchemy bound parameters) | see module 05 |
| HTML safety | `html.escape`, or a template engine with auto-escaping | escape at the output |
| Rate limit | hand-written token bucket | injected clock makes it testable |
| TLS | `uvicorn --ssl-keyfile/--ssl-certfile`, `ssl.create_default_context()` for clients | verification is on by default; never turn it off |

Idioms: small pure functions in a `security` module (`hash_password`, `verify_password`, `issue_token`, `decode_token`), and **FastAPI dependencies** (not decorators, not global middleware) for "who is calling" and "may they". Dependencies compose: `require_role("admin")` builds on `current_user`, and tests can override them with `app.dependency_overrides`. PyJWT is the one third-party library here because the stdlib has no JWT support.

## 3. Guided Examples

All examples share [examples/m11_security.py](./examples/m11_security.py) (hashing, API keys, JWT helpers, `RateLimiter`).

### 3.1 Password hashing

- **Goal:** store passwords safely and verify them in constant time.
- **Run:** `python modules/11-auth-and-security-basics/examples/01_password_hashing.py`
- **Code:** [examples/01_password_hashing.py](./examples/01_password_hashing.py)
- **Walkthrough:** `hash_password` draws a 16-byte random salt, derives 32 bytes with `scrypt` (n=2^14, about 16 MiB of memory, tens of milliseconds) and stores `scrypt$n$r$p$salt$hash`. Because the parameters are in the string, you can raise the cost later and still verify old hashes. `verify_password` returns `False` (not an exception) for malformed input and compares with `hmac.compare_digest`.

```python
digest = hashlib.scrypt(password.encode(), salt=salt, n=SCRYPT_N, r=SCRYPT_R, p=SCRYPT_P, dklen=32)
return hmac.compare_digest(actual, expected)
```

- **Expected output:**

  ```text
  scheme=scrypt n=16384 r=8 p=1
  salt chars=24 hash chars=44
  same password, different hashes: True
  right password verifies: True
  wrong password verifies: False
  garbage hash verifies: False
  compare_digest: False
  fixed salt is reproducible: True
  ```

### 3.2 JWT, and the attacks it must survive

- **Goal:** issue a token and reject every forged or stale one.
- **Run:** `python modules/11-auth-and-security-basics/examples/02_jwt_tokens.py`
- **Code:** [examples/02_jwt_tokens.py](./examples/02_jwt_tokens.py)
- **Walkthrough:** `decode_token` passes `algorithms=["HS256"]` (the token's own `alg` header is never trusted, which defeats `alg: none` and algorithm-confusion attacks) and `options={"require": ["exp", "sub"]}` (a token without expiry is invalid). PyJWT checks `exp` against the clock itself. The last line shows why secrets do not belong in claims: anyone can read the payload.
- **Expected output:**

  ```text
  valid token: accepted sub=7 role=admin
  wrong secret: rejected (InvalidSignatureError)
  forged payload: rejected (InvalidSignatureError)
  expired: rejected (ExpiredSignatureError)
  no exp claim: rejected (MissingRequiredClaimError)
  alg=none: rejected (InvalidAlgorithmError)
  payload is only encoded, not encrypted: role=user
  ```

### 3.3 SQL injection and XSS

- **Goal:** see the vulnerability, then the one-line fix.
- **Run:** `python modules/11-auth-and-security-basics/examples/03_injection_and_xss.py`
- **Code:** [examples/03_injection_and_xss.py](./examples/03_injection_and_xss.py)
- **Walkthrough:** building SQL with an f-string lets `x' OR '1'='1` change the query and return everyone's tasks. With a `?` placeholder the driver sends the value separately and it can never become SQL. For HTML, `html.escape` turns `<` `>` `&` and quotes into entities at the point of output. JSON APIs escape for you; the risk is in HTML you build yourself.
- **Expected output:**

  ```text
  vulnerable, owner=ana: ['Write docs']
  vulnerable, attack:    ['Write docs', 'Pay rent']
  safe, attack:          []
  <li>&lt;script&gt;alert(&#x27;x&#x27;)&lt;/script&gt;</li>
  <li>Fish &amp; chips</li>
  ```

### 3.4 Rate limiting with a token bucket

- **Goal:** allow bursts, bound the sustained rate, tell clients when to retry.
- **Run:** `python modules/11-auth-and-security-basics/examples/04_rate_limit.py`
- **Code:** [examples/04_rate_limit.py](./examples/04_rate_limit.py), limiter in [examples/m11_security.py](./examples/m11_security.py)
- **Walkthrough:** each key owns a bucket of `burst` tokens that refills at `refill_per_second`. A request takes one token; with none left the limiter returns the seconds until one is back, which becomes `Retry-After`. The clock is injected, so the demo (and the tests) need no sleeping.
- **Expected output:**

  ```text
  t=+0.0s request 1: allowed
  t=+0.0s request 2: allowed
  t=+0.0s request 3: allowed
  t=+0.0s request 4: 429, Retry-After: 1
  t=+0.0s request 5: 429, Retry-After: 1
  other user is unaffected: True
  t=+2.5s: allowed
  t=+2.5s: allowed
  t=+2.5s: denied
  ```

### 3.5 Task Tracker API: login, roles, API keys, rate limit

- **Goal:** all pieces in one FastAPI app.
- **Run:** `ADMIN_PASSWORD=admin-secret-1 ADDR=127.0.0.1:8080 python modules/11-auth-and-security-basics/examples/05_secure_tracker_api.py` (Ctrl-C or SIGTERM to stop). The JWT secret is random per start unless `JWT_SECRET` is set; no default credentials exist, the admin is seeded only from `ADMIN_PASSWORD`.
- **Code:** [examples/05_secure_tracker_api.py](./examples/05_secure_tracker_api.py)
- **Walkthrough:** `current_user` reads the bearer token and turns every `jwt.InvalidTokenError` into the same `401`; `require_role("admin")` wraps it and raises `403`. Login returns the same message for an unknown email and a wrong password, and verifies against a dummy hash for unknown emails so the response time does not reveal which emails exist. `POST /tasks` is limited per user (burst 3, one token per 2 s). API keys are shown once and only their SHA-256 digest is stored (keys are long random strings, so a fast hash is fine; passwords are not).

```python
def require_role(role: str) -> Callable[[User], User]:
    def check(user: Annotated[User, Depends(current_user)]) -> User:
        if user.role != role:
            raise ApiError(403, "forbidden", f"requires role {role}")
        return user

    return check
```

- **Expected output** (`$` lines are commands; `TOKEN` and `ADMIN` hold the `token` returned by logging in as ana and as `admin@example.com`; some headers omitted):

  ```console
  $ curl -si -X POST -H 'Content-Type: application/json' -d '{"email":"ana@example.com","password":"s3cret-pass"}' localhost:8080/auth/register
  HTTP/1.1 201 Created
  content-type: application/json

  {"id":2,"email":"ana@example.com"}
  $ curl -si -X POST -H 'Content-Type: application/json' -d '{"email":"bob@example.com","password":"short"}' localhost:8080/auth/register
  HTTP/1.1 422 Unprocessable Content
  content-type: application/json

  {"error":{"code":"validation_failed","message":"invalid input","fields":{"password":"String should have at least 8 characters"}}}
  $ curl -si -X POST -H 'Content-Type: application/json' -d '{"email":"nobody@example.com","password":"wrong-pass"}' localhost:8080/auth/login
  HTTP/1.1 401 Unauthorized
  www-authenticate: Bearer
  content-type: application/json

  {"error":{"code":"unauthorized","message":"invalid email or password"}}
  $ curl -s -H "Authorization: Bearer $TOKEN" localhost:8080/me
  {"id":2,"email":"ana@example.com","role":"user"}
  $ curl -s -H "Authorization: Bearer ${TOKEN}x" localhost:8080/me
  {"error":{"code":"unauthorized","message":"invalid or expired token"}}
  $ for i in 1 2 3 4; do curl -s -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{"title":"Write docs"}' localhost:8080/tasks; echo; done
  {"id":1,"title":"Write docs","owner_id":2}
  {"id":2,"title":"Write docs","owner_id":2}
  {"id":3,"title":"Write docs","owner_id":2}
  {"error":{"code":"rate_limited","message":"too many requests"}}
  $ curl -s -X DELETE -H "Authorization: Bearer $TOKEN" localhost:8080/tasks/1
  {"error":{"code":"forbidden","message":"requires role admin"}}
  $ curl -si -X DELETE -H "Authorization: Bearer $ADMIN" localhost:8080/tasks/1
  HTTP/1.1 204 No Content
  ```

  The fourth `POST /tasks` also carries `retry-after: 2`. `POST /api-keys` returns `{"api_key":"tt_..."}` once; `GET /service/ping` with that key in `X-API-Key` answers `{"owner_id":2}`, with a wrong key `401`.

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `if token == expected:` or `hash1 == hash2` | `==` is the obvious comparison and returns at the first differing byte | `hmac.compare_digest(a, b)` for secrets, MACs, API-key digests |
| `random.choice` / `uuid.uuid4().hex[:8]` / `time.time()` for tokens or salts | `random` is a seeded Mersenne Twister: predictable | `secrets.token_urlsafe(32)`, `secrets.token_bytes(16)` |
| `jwt.decode(token, key)` without `algorithms=[...]` (or `verify_signature=False` "to read claims") | Older tutorials omit it | Always pass `algorithms=["HS256"]` and `options={"require": ["exp", "sub"]}`; never use `verify_signature=False` for decisions |
| Tokens that never expire, or live for weeks | Avoids re-login logic | Short `exp` (minutes to 1 h); refresh tokens are a separate, revocable mechanism |
| Hardcoded `SECRET = "changeme"` committed to the repo | It is "just for dev" | Read from the environment (module 09), generate when absent in dev, fail fast in production; use at least 32 random bytes (PyJWT warns about short HMAC keys) |
| `hashlib.sha256(password)` or `md5` as the "hash" | Hashing is hashing, right? | Fast hashes are brute-forced at billions/s. Use `scrypt` (or `argon2`/`bcrypt` where allowed) with a per-user salt |
| `f"... WHERE owner = '{owner}'"` | f-strings are everywhere | Placeholders: `db.execute("... WHERE owner = ?", (owner,))`. Only identifiers (table/column names) cannot be bound: allow-list them |
| Different errors/timing for "unknown user" vs "wrong password" | Helpful messages | One message, and verify against a dummy hash so timing matches (user enumeration) |
| Forgetting `Depends(current_user)` on a newly added route | In FastAPI auth is opt-in per route | Put it on an `APIRouter(dependencies=[Depends(...)])` for whole groups, and test that unauthenticated calls get 401 |
| Trusting the `X-Forwarded-For` header for the rate-limit key | It looks like the client IP | Limit by authenticated user id; honour forwarded headers only from your own proxy (`uvicorn --forwarded-allow-ips`) |
| `pydantic` accepts any `str` for e-mail or password length | Types are not validation | Constrain with `Field(min_length=..., max_length=..., pattern=...)`; cap lengths (hashing a 10 MB "password" is a DoS) |
| Logging the `Authorization` header or the login body | Request logging prints everything | Log method, path, status, user id only (module 10) |

## 5. Coming From Other Languages

- **Java/C# (Spring Security, ASP.NET `[Authorize]`):** there is no framework-wide filter chain. A FastAPI `Depends` function is the unit of policy; compose them (`current_user` -> `require_role`). There are no annotations like `[Authorize(Roles=...)]`, but `Annotated[User, Depends(...)]` in the signature plays the same role.
- **Go:** auth middleware wraps a handler and puts the user in `context.Context`; in FastAPI the dependency returns the user straight into the handler parameter, with no context keys or type assertions.
- **JavaScript/TypeScript (jsonwebtoken, passport):** the same JWT pitfalls apply (`algorithms`, `exp`). `bcrypt` is common in Node; Python's stdlib ships `scrypt`, so no native dependency is needed.
- **Ruby/Rails:** `has_secure_password` hides hashing; here you build the 20 lines yourself so you know what is stored.

## 6. Exercises

Each exercise has starter code and tests in [exercises/](./exercises/). Solutions are in [solutions/](./solutions/).

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | [Password hashing](./exercises/ex1_passwords/m11_passwords.py) | Easy | `scrypt`, salts, `compare_digest`, rehash | `pytest modules/11-auth-and-security-basics/exercises/ex1_passwords` |
| 2 | [Token-bucket limiter](./exercises/ex2_token_bucket/m11_rate_limit.py) | Medium | rate limiting, injected clock | `pytest modules/11-auth-and-security-basics/exercises/ex2_token_bucket` |
| 3 | [Auth dependencies](./exercises/ex3_auth_deps/m11_auth_deps.py) | Stretch | JWT verification, `Depends`, 401 vs 403 | `pytest modules/11-auth-and-security-basics/exercises/ex3_auth_deps` |

**Exercise 1: Password hashing**

- **Task:** implement `hash_password`, `verify_password` (uses the parameters stored in the hash, constant-time compare, never raises) and `needs_rehash`.
- **Acceptance:** all tests in `test_m11_passwords.py`, including the reference computation with a fixed salt and the malformed-input table.
- <details><summary>Hint</summary>`stored.split("$")` gives six parts; wrap parsing and `hashlib.scrypt` in one `try/except ValueError` (`binascii.Error` is a `ValueError`).</details>

**Exercise 2: Token-bucket limiter**

- **Task:** implement `TokenBucketLimiter.allow` (cost, retry-after, remaining), `prune` and argument validation.
- **Acceptance:** all tests in `test_m11_rate_limit.py`: burst, refill capped at the burst, per-key isolation, denied requests take nothing.
- <details><summary>Hint</summary>Store `(tokens, updated)` per key; on every call add `(now - updated) * rate`, cap at `burst`, then decide.</details>

**Exercise 3: Auth dependencies**

- **Task:** implement `current_principal` (bearer token to `Principal`, pinned algorithm, required claims, uniform `401`) and `require_role(*roles)` (`403` for the wrong role).
- **Acceptance:** all tests in `test_m11_auth_deps.py`: nine kinds of bad token get the same `401` body; the right role passes.
- <details><summary>Hint</summary>Catch `jwt.InvalidTokenError` (plus `ValueError` for `int(sub)`) and raise one `AuthError`; `require_role` returns an inner function that has `Depends(current_principal)` in its signature.</details>

## 7. Recap

- Passwords: salted `scrypt`, parameters stored with the hash, `hmac.compare_digest`. Randomness for security: `secrets`.
- JWT: pinned `algorithms`, required `exp`/`sub`, short lifetime, no secrets in claims.
- FastAPI auth is `Depends`: one dependency identifies the caller, another checks the role; `401` = who are you, `403` = not allowed.
- Bind SQL parameters, escape HTML at output, know when CSRF applies (cookies), terminate TLS at a trusted hop, rate-limit with a token bucket.
- Reach for the stdlib for hashing and randomness, PyJWT for tokens, and a hosted identity provider when requirements grow beyond a login form.

## 8. Self-check Questions

1. Why is `scrypt` with a per-user salt better than `sha256(password)`, and why must `n`, `r`, `p` be stored with the hash?
2. A token header says `{"alg": "none"}`. What stops your API from accepting it?
3. A user submits a valid token for a user that has since been deleted. Which status code, and where in the code is it decided?
4. Why does `POST /auth/login` run a hash even when the e-mail is unknown?
5. Your API uses only `Authorization: Bearer` headers. Is it vulnerable to CSRF? What changes if the token is stored in a cookie?

## 9. Further Reading

- [OWASP Cheat Sheet Series](https://cheatsheetseries.owasp.org/): Password Storage, JWT, SQL Injection Prevention, CSRF Prevention, XSS Prevention
- [FastAPI: Security](https://fastapi.tiangolo.com/tutorial/security/) and [PyJWT usage](https://pyjwt.readthedocs.io/en/stable/usage.html)
- [hashlib.scrypt](https://docs.python.org/3/library/hashlib.html#hashlib.scrypt), [secrets](https://docs.python.org/3/library/secrets.html), [hmac.compare_digest](https://docs.python.org/3/library/hmac.html#hmac.compare_digest)

---
**Next:** [12 Testing](../12-testing/README.md)
