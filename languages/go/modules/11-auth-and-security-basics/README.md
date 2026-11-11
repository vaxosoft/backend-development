# 11. Authentication and Security Basics in Go

> Estimated time: 2h | Difficulty: Intermediate | Prerequisites: 02, 03, 05, 10

## Learning Outcomes
After this module you can:
- [ ] Hash and verify passwords with bcrypt, generate secrets with `crypto/rand` and compare them in constant time
- [ ] Issue and verify HS256 JWTs, pinning the algorithm, issuer and expiry
- [ ] Write authentication middleware, role checks and per-resource ownership checks
- [ ] Explain and prevent SQL injection, XSS and CSRF; use sessions in cookies with the right flags
- [ ] Rate-limit per user with a token bucket and serve over TLS

## 1. Concept Overview
**Authentication** answers *who is calling* (password, session cookie, API key, token); **authorization** answers *may they do this* (roles, ownership). Never store a password: store a slow, salted hash. Never trust input: keep data and code apart (parameterised SQL, context-aware HTML escaping) and validate against an allow-list. Browsers add ambient credentials (cookies), so state-changing requests also need CSRF protection. Rate limits and TLS protect the service and the credentials in transit.

```
request -> [rate limit] -> [authenticate: token/cookie -> principal in ctx] -> [authorize: role / owner] -> handler
                401 = who are you?            403 = known, but not allowed            429 = slow down
```

## 2. How Go Approaches It

| Need | Tool |
|------|------|
| Password hashing | `golang.org/x/crypto/bcrypt` (`GenerateFromPassword`, `CompareHashAndPassword`, 72-byte limit) |
| Random secrets / constant-time compare | `crypto/rand`, `crypto/subtle.ConstantTimeCompare`, `crypto/sha256` for stored API keys |
| JWT | `github.com/golang-jwt/jwt/v5`: `ParseWithClaims` with `WithValidMethods`, `WithIssuer`, `WithExpirationRequired` |
| Middleware | `func(http.Handler) http.Handler`; the principal travels in `context.Context` under an unexported key type |
| Sessions, CSRF | `http.SetCookie` (`HttpOnly`, `Secure`, `SameSite`), server-side session map, per-session CSRF token |
| Injection / XSS | `database/sql` placeholders; `html/template` (never `text/template`) for HTML |
| Rate limiting | `golang.org/x/time/rate` token bucket; one limiter per key |
| TLS | `crypto/tls`, `srv.ListenAndServeTLS`, `httptest.NewTLSServer` in tests |

Idiomatic Go keeps this explicit: small middleware you can read in one screen, no framework magic, verification parameters spelled out at the call site. Non-idiomatic: a global "current user", hand-rolled crypto, parsing JWTs by splitting on `.`.

## 3. Guided Examples
Run from `languages/go/`.

### 3.1 Passwords, tokens, constant-time compare
- **Goal:** store passwords and API keys safely.
- **Run:** `go run ./modules/11-auth-and-security-basics/examples/01_passwords_and_secrets`
- **Code:** [examples/01_passwords_and_secrets/main.go](examples/01_passwords_and_secrets/main.go)
  ```go
  hash, _ := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
  err := bcrypt.CompareHashAndPassword(hash, []byte(attempt)) // errors.Is(err, bcrypt.ErrMismatchedHashAndPassword)
  subtle.ConstantTimeCompare(fingerprint(presented), stored) == 1
  ```
- **Walkthrough:** the hash embeds algorithm, cost and a random salt (`$2a$10$...`, 60 bytes), so equal passwords hash differently and verification needs only the hash. bcrypt refuses passwords over 72 bytes (`ErrPasswordTooLong`) instead of silently truncating. API keys are long and random, so one SHA-256 is enough for storage; compare fingerprints with `ConstantTimeCompare` so response time does not leak a matching prefix.
- **Expected output:**
  ```
  hash prefix: $2a$10$
  hash length: 60
  cost: 10
  same password, different hashes: true
  right password: <nil>
  wrong password is mismatch: true
  73-byte password too long: true
  api key length: 64
  two tokens differ: true
  presented key accepted: true
  guessed key accepted: false
  ```

### 3.2 JWT: issue and verify
- **Goal:** accept only tokens you signed, with the claims you require.
- **Run:** `go run ./modules/11-auth-and-security-basics/examples/02_jwt`
- **Code:** [examples/02_jwt/main.go](examples/02_jwt/main.go)
  ```go
  jwt.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) { return secret, nil },
      jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(),
      jwt.WithIssuer("tasktracker"), jwt.WithTimeFunc(clock))
  ```
- **Walkthrough:** a JWT is `header.payload.signature`; the payload is only base64, so it is **readable by anyone**: put no secrets in it. The signature stops tampering. `WithValidMethods` rejects `alg: none` and other algorithms, `WithExpirationRequired` rejects tokens that never expire, and `WithTimeFunc` makes expiry testable. Failures are matched with `errors.Is(err, jwt.ErrTokenExpired)` etc.
- **Expected output:**
  ```
  parts: 3
  payload is only encoded, not secret: {"role":"member","iss":"tasktracker","sub":"42","exp":1893492000,"iat":1893488400}
  valid: 42 member <nil>
  expired: true
  tampered payload: true
  alg none rejected: true
  other alg rejected: true
  wrong issuer: true
  missing exp: true
  ```

### 3.3 Untrusted input: SQL injection and XSS
- **Goal:** see the attacks work against the wrong code, fail against the right code.
- **Run:** `go run ./modules/11-auth-and-security-basics/examples/03_untrusted_input`
- **Code:** [examples/03_untrusted_input/main.go](examples/03_untrusted_input/main.go)
  ```go
  db.Query("SELECT name FROM users WHERE email = '"+email+"'")  // injectable
  db.Query("SELECT name FROM users WHERE email = ?", email)      // data stays data
  ```
- **Walkthrough:** the payload `nobody' OR '1'='1` turns the concatenated query into "all rows"; as a placeholder argument it is just an email that does not exist. `html/template` escapes by context (HTML text, attribute, URL) and neutralises `javascript:` URLs (`#ZgotmplZ`); `text/template` prints the script tag verbatim. Tags are validated against an allow-list regexp.
- **Expected output:**
  ```
  concatenated, attack: [Ada Linus]
  parameterised, attack: []
  parameterised, honest: [Ada]
  concatenated, stray quote breaks the query: true
  text/template: <h1><script>alert("xss")</script></h1><a href="javascript:alert(1)">link</a>
  html/template: <h1>&lt;script&gt;alert(&#34;xss&#34;)&lt;/script&gt;</h1><a href="#ZgotmplZ">link</a>
  tag "docs" valid: true
  tag "p1" valid: true
  tag "Docs" valid: false
  tag "a b" valid: false
  tag "" valid: false
  tag "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx" valid: false
  ```

### 3.4 Sessions in cookies and CSRF
- **Goal:** cookie sessions that survive a cross-site attack.
- **Run:** `go run ./modules/11-auth-and-security-basics/examples/04_sessions_and_csrf`
- **Code:** [examples/04_sessions_and_csrf/main.go](examples/04_sessions_and_csrf/main.go)
  ```go
  http.SetCookie(w, &http.Cookie{Name: "sid", Value: id, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode})
  // unsafe methods: X-CSRF-Token must equal the session's token (ConstantTimeCompare)
  ```
- **Walkthrough:** the cookie holds only a random session id; the data lives on the server, so logout (delete + expire the cookie) really ends the session. `HttpOnly` hides it from scripts, `SameSite=Lax` keeps it off cross-site POSTs, and `Secure` (off here because the demo is plain HTTP) keeps it off cleartext. The CSRF token is not a cookie, so a forged cross-site form cannot supply it. The program drives a real `httptest` server through a `cookiejar` client.
- **Expected output:**
  ```
  me before login          401 unauthorized
  login wrong password     401 invalid credentials
  login cookie: name=sid httponly=true samesite-lax=true path=/
  me after login           200 hello ada
  post without csrf        403 csrf token missing or wrong
  post with wrong csrf     403 csrf token missing or wrong
  post with csrf           201 task created by ada
  logout                   204 
  me after logout          401 unauthorized
  ```

### 3.5 Task Tracker: a secure API
- **Goal:** everything together: register, login, JWT middleware, roles, ownership, rate limit, headers, optional TLS.
- **Run:** `ADDR=127.0.0.1:8080 go run ./modules/11-auth-and-security-basics/examples/05_secure_task_api` (Ctrl-C stops it; `RATE_BURST` defaults to 3, `RATE_PER_SEC` to 1; an admin `admin@example.com` / `admin-demo-pass` is seeded for the demo only)
- **Code:** [examples/05_secure_task_api/main.go](examples/05_secure_task_api/main.go)
  ```go
  mux.Handle("POST /tasks", a.authenticate(a.limit(http.HandlerFunc(a.createTask))))
  mux.Handle("GET /admin/users", a.authenticate(requireRole(roleAdmin, http.HandlerFunc(a.listUsers))))
  ```
- **Walkthrough:** `authenticate` verifies the bearer token and stores a `principal` in the context; `requireRole` and the owner check in `deleteTask` decide 403; login runs bcrypt against a dummy hash for unknown emails and returns the same 401 message either way. `limit` keeps one `rate.Limiter` per user id and cancels the reservation of rejected requests. Every response carries `nosniff`, `no-store` and a locked-down CSP.
- **Try it** (second terminal; `TOKEN` extraction uses `sed`):
  ```bash
  B=http://127.0.0.1:8080; J='Content-Type: application/json'
  curl -s -XPOST $B/auth/register -H "$J" -d '{"email":"ada@example.com","password":"s3cret-pass"}'
  TOKEN=$(curl -s -XPOST $B/auth/login -H "$J" -d '{"email":"ada@example.com","password":"s3cret-pass"}' | sed 's/.*"token":"\([^"]*\)".*/\1/')
  curl -si $B/tasks                                                              # no token
  curl -si -XPOST $B/tasks -H "Authorization: Bearer $TOKEN" -H "$J" -d '{"title":"Write docs"}'
  for i in 1 2 3; do curl -s -o /dev/null -w '%{http_code}\n' -XPOST $B/tasks -H "Authorization: Bearer $TOKEN" -H "$J" -d '{"title":"t"}'; done
  curl -s $B/admin/users -H "Authorization: Bearer $TOKEN"                       # member -> 403
  ```
- **Expected output** (headers trimmed to the interesting ones; the three loop codes are `201 201 429` because the first task used one of the 3 burst tokens; the `429` carries `Retry-After: 1`):
  ```
  {"email":"ada@example.com","id":2}
  HTTP/1.1 401 Unauthorized                       (+ WWW-Authenticate: Bearer)
  {"error":{"code":"unauthorized","message":"missing or invalid credentials"}}
  HTTP/1.1 201 Created                            (+ Location: /tasks/1, X-Content-Type-Options: nosniff)
  {"id":1,"title":"Write docs","status":"todo","owner_id":2}
  201
  201
  429
  {"error":{"code":"forbidden","message":"not allowed"}}
  ```
- **TLS:** `openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 -nodes -keyout key.pem -out cert.pem -days 1 -subj "/CN=localhost" -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"`, then run with `TLS_CERT=cert.pem TLS_KEY=key.pem` and call `curl --cacert cert.pem https://127.0.0.1:8080/tasks` (without `--cacert` curl rejects the self-signed certificate as an unknown authority; the demo certificate is for local use only). `tls.Config{MinVersion: tls.VersionTLS12}` is set on the server.

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| Trusting the JWT `alg` header | The keyfunc returns the key without looking at the token's method; `none` or a swapped algorithm gets through | `jwt.WithValidMethods([]string{"HS256"})`, also require `exp` and the issuer |
| `==` on secrets | Convenient and wrong: timing reveals how many leading bytes matched | `subtle.ConstantTimeCompare` (compare equal-length hashes, not raw input) |
| `math/rand` for tokens | The name is familiar and the output looks random | `crypto/rand` (`rand.Read`), never `math/rand` for secrets |
| bcrypt silently ignoring bytes past 72 | Very long passwords/passphrases share a prefix | Reject over 72 bytes (`ErrPasswordTooLong`); do not pre-hash without a plan |
| Fast hash (SHA-256, MD5) for passwords | They are quick for attackers too | bcrypt with a cost you can afford (10+); re-hash when the cost rises |
| `fmt.Sprintf` into SQL | String building is the first thing everyone learns | `?` / `$1` placeholders; only identifiers need an allow-list |
| `text/template` for HTML | Same API as `html/template`, no escaping | Import `html/template` for anything a browser renders |
| 401 vs 403 mixed up, or different messages for "no such user" and "wrong password" | Helpful errors become an account-enumeration oracle | 401 = unauthenticated, 403 = not allowed; one generic login failure, dummy bcrypt for unknown users |
| Shared `rate.Limiter` for all users, or `Allow()` that borrows from the future | One bucket punishes everyone; `Reserve` without `Cancel` pushes the wait out on every rejected try | A limiter per key; `Reserve` then `Cancel` when the delay is non-zero |
| Cookie without `HttpOnly`/`SameSite`/`Secure`, or session id in the URL | `http.Cookie{}` defaults are permissive | Set all three flags; random 128+ bit ids; rotate on login |
| Context key of type `string` | `ctx.Value("user")` can collide across packages | Unexported `type ctxKey struct{}` plus a typed accessor |

## 5. Coming From Other Languages
- **Java/C# (Spring Security, ASP.NET `[Authorize]`):** no annotations or filters configured by convention: auth is ordinary middleware wrapped around handlers, and you pass the principal through `context`. JWT validation parameters are explicit options, not a global config object.
- **Python/JS (Django, Express + passport):** there is no session or user object on the request; you build both. `bcrypt` and `jwt` are libraries, not framework features, and templates escape only when you use `html/template`.
- **Rust:** the same crypto building blocks exist as crates; Go's `errors.Is` plays the role of matching on the error enum, and constant-time helpers live in `crypto/subtle`.

## 6. Exercises
Starter code and tests are in [exercises/](exercises/); reference solutions in [_solutions/](_solutions/).

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Password and token toolkit | Easy | bcrypt rules, sentinel errors, `crypto/rand`, constant time | `go test ./modules/11-auth-and-security-basics/exercises/ex1_password_and_tokens/` |
| 2 | JWT service | Medium | pinned algorithm, expiry, issuer, injectable clock | `go test ./modules/11-auth-and-security-basics/exercises/ex2_jwt_tokens/` |
| 3 | Per-key rate limiter | Stretch | token bucket, `Reserve`/`Cancel`, `Retry-After`, concurrency | `go test -race ./modules/11-auth-and-security-basics/exercises/ex3_rate_limiter/` |

### Exercise 1: Password and token toolkit
- **Task:** implement `Hasher` (`Hash`, `Verify`, `NeedsRehash`) with the length rules, plus `NewToken`, `HashToken` and `TokenEqual`.
- **Acceptance:** `TestHashIsSaltedAndNotPlaintext`, `TestVerify`, `TestPasswordRules`, `TestNeedsRehash`, `TestNewToken`, `TestHashToken`, `TestTokenEqual`.
- <details><summary>Hint</summary>Map <code>bcrypt.ErrMismatchedHashAndPassword</code> to your own <code>ErrMismatch</code>, and use <code>bcrypt.Cost</code> for <code>NeedsRehash</code>.</details>

### Exercise 2: JWT service
- **Task:** implement `Service.Issue` and `Service.Verify` so that only correctly signed HS256 tokens with the right issuer and an expiry are accepted; expired tokens return `ErrExpired`, everything else `ErrInvalid`.
- **Acceptance:** `TestRoundTrip`, `TestIssuedTokenShape`, `TestShortSecretIsRefused`, `TestExpiry`, `TestRejectsUntrustworthyTokens` (alg `none`, HS512, tampered, wrong secret/issuer, no `exp`), `TestZeroNowUsesRealClock`.
- <details><summary>Hint</summary>Check <code>errors.Is(err, jwt.ErrTokenExpired)</code> before the generic failure, pass <code>jwt.WithTimeFunc(s.now)</code> and wrap other errors with <code>%w</code> of <code>ErrInvalid</code>.</details>

### Exercise 3: Per-key rate limiter
- **Task:** implement `Limiter.Allow` (one bucket per key, injectable clock, rejected calls consume nothing) and the `Middleware` that answers `429` with `Retry-After` and the error envelope.
- **Acceptance:** `TestBurstThenRefill`, `TestRejectionsDoNotConsumeTokens`, `TestPartialRefillReportsRemainingWait`, `TestKeysAreIndependent`, `TestBucketNeverExceedsBurst`, `TestConcurrentCallsShareOneBucket`, `TestMiddleware`.
- <details><summary>Hint</summary>Use <code>rate.Limiter.ReserveN(now, 1)</code>; if <code>DelayFrom(now) &gt; 0</code> call <code>CancelAt(now)</code> and report the delay. Guard the map with a mutex.</details>

## 7. Recap
- Hash passwords with bcrypt, store high-entropy tokens as SHA-256, generate secrets with `crypto/rand`, compare with `subtle`.
- Verify JWTs with an explicit algorithm, issuer and required expiry; the payload is public.
- Authenticate in middleware (401), authorize by role and ownership (403), rate-limit per user (429).
- Parameterise SQL, escape HTML with `html/template`, protect cookie sessions with flags and a CSRF token.
- Reach for bearer tokens for API clients and `HttpOnly` cookie sessions plus CSRF for browsers; terminate TLS (`MinVersion` 1.2+) at the server or a proxy.

## 8. Self-check Questions
1. Why does `bcrypt.GenerateFromPassword` return a different hash for the same password, and how can `CompareHashAndPassword` still work?
2. A token arrives with `"alg":"none"` and a valid-looking payload. Which option stops it, and what happens if your keyfunc ignores the method?
3. Why is it safe to log a JWT's `sub` but unsafe to put a user's email-verification secret in its payload?
4. What does `SameSite=Lax` stop, and what does it not stop that the CSRF token does?
5. In 3.5, why does the login handler run bcrypt even when the email is unknown?

## 9. Further Reading
- [OWASP Cheat Sheets](https://cheatsheetseries.owasp.org/): Password Storage, Session Management, CSRF Prevention, SQL Injection Prevention
- [golang-jwt/jwt v5](https://pkg.go.dev/github.com/golang-jwt/jwt/v5), [x/crypto/bcrypt](https://pkg.go.dev/golang.org/x/crypto/bcrypt), [x/time/rate](https://pkg.go.dev/golang.org/x/time/rate)
- [html/template security model](https://pkg.go.dev/html/template#hdr-Security_Model) and [crypto/tls](https://pkg.go.dev/crypto/tls)

---
**Next:** [12 Testing](../12-testing/README.md)
