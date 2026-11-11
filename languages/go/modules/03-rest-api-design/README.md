# 03. REST API Design in Go

> Estimated time: 2h | Difficulty: Intermediate | Prerequisites: 01, 02

## Learning Outcomes
After this module you can:
- [ ] Model resources and pick the right verb and status code (200/201/204/400/404/405/413/422)
- [ ] Return every failure in one JSON error envelope, including per-field validation errors
- [ ] Separate decoding problems (400/413) from business-rule problems (422)
- [ ] Implement filtering, sorting and clamped offset pagination with a stable response shape
- [ ] Version an API with a path prefix, make POST retry-safe with `Idempotency-Key`, and publish an OpenAPI document

## 1. Concept Overview
REST over HTTP means: **nouns in URLs** (`/v1/tasks/42`), **verbs in methods** (`GET` reads, `POST` creates, `PUT` replaces, `PATCH` changes part, `DELETE` removes), and **status codes** that tell clients what happened without parsing text. Clients can only be generic if errors are uniform, so one envelope is used everywhere:

```json
{"error":{"code":"validation_failed","message":"invalid task","fields":{"title":"must be 1-200 characters"}}}
```

| Situation | Status | `code` |
|-----------|--------|--------|
| created (with `Location`) / read / updated / deleted | 201 / 200 / 200 / 204 | |
| malformed JSON, bad query value | 400 | `bad_request` |
| unknown resource | 404 | `not_found` |
| payload too large | 413 | `payload_too_large` |
| well-formed but invalid data | 422 | `validation_failed` (+ `fields`) |
| unexpected | 500 | `internal` (never leak details) |

PUT and DELETE are idempotent by definition (repeating them leaves the same state); POST is not, unless you add an idempotency key. Lists return `{"items":[...],"total":N,"limit":L,"offset":O}`; `limit` defaults to 20 and is clamped to 100.

## 2. How Go Approaches It
- **No framework.** `net/http`'s `ServeMux` (method + path patterns), `encoding/json` and a few small helper functions are enough; chi/gin are intentionally skipped.
- **Helpers, not annotations.** `writeJSON(w, status, v)` and `writeError(w, status, code, msg, fields)` are the whole "framework". Handlers decode, validate, call a store, and write.
- **Validation is plain code** returning `map[string]string` (field -> problem). Typed errors (`*ValidationError`) plus `errors.As` let a single `WriteErr` map errors to responses.
- **Absent vs null for PATCH:** pointers distinguish "absent" from a value but not from `null`; a tiny generic `optional[T]` with `UnmarshalJSON` does (example 3.5).
- **405 in JSON:** register a method-less pattern next to the method patterns (`mux.HandleFunc("/tasks/{id}", methodNotAllowed(...))`); the more specific `GET /tasks/{id}` still wins.
- **OpenAPI:** the stdlib has no generator; the idiomatic lightweight option is a hand-written `openapi.json` embedded with `//go:embed` and served.

Run everything from `languages/go/`. Servers honour `ADDR`; the outputs below were captured with `ADDR=127.0.0.1:20001` (3.1) and `20002` (3.5), using `curl -s -w '[%{http_code}]\n'` to print the status.

## 3. Guided Examples

### 3.1 Resources, status codes and the envelope
- **Goal:** the right status for each outcome, one envelope for all errors.
- **Run:** `ADDR=127.0.0.1:20001 go run ./modules/03-rest-api-design/examples/01_status_and_envelope`
- **Code:** [examples/01_status_and_envelope/main.go](examples/01_status_and_envelope/main.go)
  ```go
  w.Header().Set("Location", "/tasks/"+strconv.Itoa(t.ID))
  writeJSON(w, http.StatusCreated, t)
  ```
- **Walkthrough:** invalid JSON is `400`; valid JSON with an empty title is `422` with `fields`. `DELETE` answers `204` with no body, a second `DELETE` is `404`. The catch-all `/` pattern and `methodNotAllowed` make 404/405 use the envelope too (the mux's own are plain text). `405` carries an `Allow` header. (`method_not_allowed` is an extra code beyond the shared table.)
- **Expected output:**
  ```
  $ curl -si -X POST -d '{"title":"Write docs"}' localhost:20001/tasks
  HTTP/1.1 201 Created
  Content-Type: application/json
  Location: /tasks/1
  Content-Length: 30

  {"id":1,"title":"Write docs"}
  $ curl ... /tasks/abc      {"error":{"code":"bad_request","message":"id must be an integer"}} [400]
  $ curl ... -X POST -d '{"title":"  "}' /tasks
  {"error":{"code":"validation_failed","message":"invalid task","fields":{"title":"must be 1-200 characters"}}}
  [422]
  $ curl ... -X POST -d '{"title":' /tasks
  {"error":{"code":"bad_request","message":"request body is not valid JSON"}}
  [400]
  $ curl -si -X PUT /tasks/1
  HTTP/1.1 405 Method Not Allowed
  Allow: GET, DELETE
  {"error":{"code":"method_not_allowed","message":"PUT is not supported here"}}
  $ curl -si -X DELETE /tasks/1        ->  HTTP/1.1 204 No Content
  $ curl -s -X DELETE /tasks/1         ->  {"error":{"code":"not_found","message":"task 1 not found"}} [404]
  $ curl -s /nope                      ->  {"error":{"code":"not_found","message":"no such route"}} [404]
  ```

### 3.2 Validation: 400 vs 413 vs 422
- **Goal:** strict decoding separated from business validation.
- **Run:** `go run ./modules/03-rest-api-design/examples/02_validation` (in-process with `httptest`; no port)
- **Code:** [examples/02_validation/main.go](examples/02_validation/main.go)
  ```go
  r.Body = http.MaxBytesReader(w, r.Body, 512)
  dec.DisallowUnknownFields()
  if _, extra := dec.Token(); !errors.Is(extra, io.EOF) { /* trailing data */ }
  ```
- **Walkthrough:** `decode` handles *syntax* (malformed, wrong type, unknown field, trailing data -> 400; too large -> 413 via `*http.MaxBytesError`). `validate` handles *meaning* and reports **every** broken field at once, so a client fixes a form in one round trip. It also normalises (trims the title, defaults the status).
- **Expected output**:
  ```
  valid            201 {"title":"Write docs","status":"todo"}
  malformed JSON   400 {"error":{"code":"bad_request","message":"invalid JSON request body"}}
  wrong type       400 {"error":{"code":"bad_request","message":"invalid JSON request body"}}
  unknown field    400 {"error":{"code":"bad_request","message":"invalid JSON request body"}}
  trailing data    400 {"error":{"code":"bad_request","message":"invalid JSON request body"}}
  too large        413 {"error":{"code":"payload_too_large","message":"body exceeds 512 bytes"}}
  empty title      422 {"error":{"code":"validation_failed","message":"invalid task","fields":{"title":"must be 1-200 characters"}}}
  several problems 422 {"error":{"code":"validation_failed","message":"invalid task","fields":{"due_date":"must be a date in YYYY-MM-DD format","status":"must be one of todo, in_progress, done","tags":"each tag must be 1-30 lowercase characters and unique","title":"must be 1-200 characters"}}}
  ```

### 3.3 Pagination, filtering, sorting
- **Goal:** predictable list queries with clamping and stable ordering.
- **Run:** `go run ./modules/03-rest-api-design/examples/03_list_queries`
- **Code:** [examples/03_list_queries/main.go](examples/03_list_queries/main.go)
  ```go
  q.limit = min(n, maxLimit)                          // clamp, don't fail
  return cmp.Or(c, cmp.Compare(a.ID, b.ID))           // tie-break: stable pages
  ```
- **Walkthrough:** `parseQuery` applies defaults and rejects garbage with `400`. Sorting uses a stable sort with an ID tie-break (otherwise pages can repeat or skip rows); tasks without a due date sort last in both directions. An offset past the end gives `"items":[]` (never `null`) with the real `total`.
- **Expected output:**
  ```
  /tasks                               200 ids=[5 4 3 2 1] total=5 limit=20 offset=0
  /tasks?limit=2                       200 ids=[5 4] total=5 limit=2 offset=0
  /tasks?limit=2&offset=2              200 ids=[3 2] total=5 limit=2 offset=2
  /tasks?limit=2&offset=4              200 ids=[1] total=5 limit=2 offset=4
  /tasks?offset=50                     200 ids=[] total=5 limit=20 offset=50
  /tasks?status=todo&sort=due_date     200 ids=[4 1 5] total=3 limit=20 offset=0
  /tasks?status=todo&sort=-due_date    200 ids=[1 4 5] total=3 limit=20 offset=0
  /tasks?tag=p1&sort=created_at        200 ids=[2 3 4] total=3 limit=20 offset=0
  /tasks?limit=1000                    200 ids=[5 4 3 2 1] total=5 limit=100 offset=0
  /tasks?limit=abc                     400 {"error":{"code":"bad_request","message":"limit must be a positive integer"}}
  /tasks?sort=title                    400 {"error":{"code":"bad_request","message":"sort must be created_at, -created_at, due_date or -due_date"}}
  ```

### 3.4 Versioning and idempotency
- **Goal:** a `/v1` prefix, a retired `/v0`, and an `Idempotency-Key` middleware.
- **Run:** `go run ./modules/03-rest-api-design/examples/04_versioning_idempotency`
- **Code:** [examples/04_versioning_idempotency/main.go](examples/04_versioning_idempotency/main.go)
  ```go
  root.Handle("/v1/", http.StripPrefix("/v1", Idempotent(s.v1())))
  ```
- **Walkthrough:** `Idempotent` fingerprints the body (SHA-256), runs the handler once per key, stores the captured response and replays it with `Idempotent-Replayed: true`. Reusing a key with a different body is `422`. Note the header is set *before* the replayed body is written. Real systems also scope keys per user and expire them.
- **Expected output:**
  ```
  POST   /v1/tasks      key=     -> 201 {"id":1,"title":"A"} Location=/v1/tasks/1
  POST   /v1/tasks      key=     -> 201 {"id":2,"title":"A"} Location=/v1/tasks/2
  POST   /v1/tasks      key=k1   -> 201 {"id":3,"title":"B"} Location=/v1/tasks/3
  POST   /v1/tasks      key=k1   -> 201 {"id":3,"title":"B"} Location=/v1/tasks/3 Idempotent-Replayed=true
  POST   /v1/tasks      key=k1   -> 422 {"error":{"code":"idempotency_key_reused","message":"key was used with a different request body"}}
  POST   /v1/tasks      key=k2   -> 201 {"id":4,"title":"D"} Location=/v1/tasks/4
  PUT    /v1/tasks/1    key=     -> 200 {"id":1,"title":"A2"}
  PUT    /v1/tasks/1    key=     -> 200 {"id":1,"title":"A2"}
  DELETE /v1/tasks/1    key=     -> 204 
  DELETE /v1/tasks/1    key=     -> 404 {"error":{"code":"not_found","message":"task not found"}}
  GET    /v0/tasks      key=     -> 410 {"error":{"code":"gone","message":"v0 was removed, use /v1"}} Deprecation=true Link=</v1/tasks>; rel="successor-version"
  ```

### 3.5 The Task Tracker API
- **Goal:** the complete CRUD API in the shared domain: create/get/PUT/PATCH/DELETE, list, validation, envelope, OpenAPI.
- **Run:** `ADDR=127.0.0.1:20002 go run ./modules/03-rest-api-design/examples/05_tasks_api`
- **Code:** [examples/05_tasks_api/main.go](examples/05_tasks_api/main.go), [openapi.json](examples/05_tasks_api/openapi.json)
  ```go
  type optional[T any] struct{ Set bool; Value *T }   // "absent" vs "null"
  func (o *optional[T]) UnmarshalJSON(b []byte) error { o.Set = true; return json.Unmarshal(b, &o.Value) }
  ```
- **Walkthrough:** PATCH merges the sent fields into the stored task and validates the *result*; `"due_date": null` clears the date while an absent key leaves it. PUT replaces all writable fields (omitted ones reset to defaults). `created_at` comes from a demo clock that starts at `2030-01-01T09:00:00Z` and ticks one minute per task so the output is reproducible; production code would use `time.Now`. Unknown fields (e.g. `id`) are rejected with `400`.
- **Expected output** (`B=localhost:20002/v1`):
  ```
  $ curl -si -X POST -d '{"title":"Write docs","tags":["docs","p1"],"due_date":"2030-01-31"}' $B/tasks
  HTTP/1.1 201 Created
  Content-Type: application/json
  Location: /v1/tasks/1
  Content-Length: 143

  {"id":1,"title":"Write docs","status":"todo","due_date":"2030-01-31","tags":["docs","p1"],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}
  $ curl -s -X POST -d '{"title":"","status":"blocked","tags":["Docs"]}' $B/tasks
  {"error":{"code":"validation_failed","message":"invalid task","fields":{"status":"must be one of todo, in_progress, done","tags":"each tag must be 1-30 lowercase characters and unique","title":"must be 1-200 characters"}}}
  $ curl -s -X POST -d '{"title":"x","id":99}' $B/tasks
  {"error":{"code":"bad_request","message":"invalid JSON request body"}}
  $ curl -s "$B/tasks?tag=p1&sort=created_at&limit=1"
  {"items":[{"id":1,"title":"Write docs","status":"todo","due_date":"2030-01-31","tags":["docs","p1"],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}],"total":2,"limit":1,"offset":0}
  $ curl -s -X PATCH -d '{"status":"done","due_date":null}' $B/tasks/1
  {"id":1,"title":"Write docs","status":"done","due_date":null,"tags":["docs","p1"],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}
  $ curl -s -X PATCH -d '{"status":"nope"}' $B/tasks/1
  {"error":{"code":"validation_failed","message":"invalid task","fields":{"status":"must be one of todo, in_progress, done"}}}
  $ curl -s -X PUT -d '{"title":"Replaced"}' $B/tasks/2
  {"id":2,"title":"Replaced","status":"todo","due_date":null,"tags":[],"owner_id":null,"created_at":"2030-01-01T09:01:00Z"}
  $ curl -s "$B/tasks?limit=0"
  {"error":{"code":"bad_request","message":"limit must be a positive integer"}}
  $ curl -s -X DELETE $B/tasks/3  -> 204 (empty);  curl -s $B/tasks/3 -> {"error":{"code":"not_found","message":"task 3 not found"}} [404]
  $ curl -s $B/openapi.json | head -c 60
  {
    "openapi": "3.1.0",
    "info": { "title": "Task Tracker A
  ```
  Two more tasks were created right after the first one (`Review PR` with `status: in_progress`, then `Ship it` with `owner_id: 7`), hence ids 2 and 3 below.

## 4. Common Pitfalls
| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `200 OK` with `{"error":...}` | `http.Error`/`WriteHeader` forgotten; handler just encodes a struct | one `writeError(w, status, code, msg, fields)` helper that always sets the status |
| 400 vs 422 chosen at random | "bad input" feels like one category | 400 = cannot be decoded/understood; 422 = decoded fine, rules fail (with `fields`) |
| `{"items":null}` for an empty list | a nil `[]T` marshals to `null` | initialise with `[]T{}` / `make([]T, 0)` before encoding |
| Unbounded `limit` | `strconv.Atoi` result used directly | clamp with `min(n, 100)`; reject `< 1` |
| Unstable pagination | sort by a non-unique column without a tie-break | add `ID` as the last sort key; use `slices.SortStableFunc` |
| Can't tell `null` from absent in PATCH | `*string` is `nil` in both cases | custom `optional[T]` or decode into `map[string]json.RawMessage` |
| `WriteHeader` before `Header().Set` | headers freeze at the first `WriteHeader`/`Write` (e.g. `Content-Type`, `Location` lost) | set headers first, status second, body last |
| Leaking `err.Error()` in a 500 | convenient while debugging | log the error server-side, send a fixed `"internal server error"` |
| Letting the mux answer 404/405 | plain-text bodies break clients that parse JSON | catch-all `/` plus a method-less pattern per resource |
| Sending a body with `204` | habit from `writeJSON` | `w.WriteHeader(http.StatusNoContent)` and nothing else |
| Retrying `POST` blindly | `POST` is not idempotent: duplicates | `Idempotency-Key`, or make the operation a `PUT` to a client-chosen ID |

## 5. Coming From Other Languages
- **Spring / ASP.NET:** no `@Valid`, `ModelState` or `@ControllerAdvice`: validation is a function you call and the "advice" is your `WriteErr`. ASP.NET answers with `application/problem+json`; this track uses the shared envelope instead.
- **Express / Flask:** same hand-written style, but decoding into a typed struct rejects wrong types for you; `DisallowUnknownFields` is the strict-mode switch.
- **FastAPI / Pydantic:** the closest analogue to OpenAPI-from-types doesn't exist in stdlib Go; the spec is written (or generated by external tools) and embedded.
- **Rust (axum):** extractors (`Json<T>`) do decode + 4xx automatically; in Go you do it in a helper and choose the status yourself.

## 6. Exercises
Starter code and tests are in [exercises/](exercises/); solutions in [_solutions/](_solutions/) at the same relative paths.

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | [Error envelope](exercises/ex1_envelope/api.go) | Easy | JSON helpers, error -> status mapping with `errors.Is/As` | `go test ./modules/03-rest-api-design/exercises/ex1_envelope/` |
| 2 | [List queries](exercises/ex2_list_query/listing.go) | Medium | query validation, clamping, stable sort, generic pagination | `go test ./modules/03-rest-api-design/exercises/ex2_list_query/` |
| 3 | [Create endpoint](exercises/ex3_create_task/create.go) | Stretch | strict decode, 400/413/422/500, `Location`, normalisation | `go test ./modules/03-rest-api-design/exercises/ex3_create_task/` |

**Exercise 1: Error envelope**
- **Task:** implement `WriteJSON`, `WriteError` (omit empty `fields`) and `WriteErr`, which maps `*ValidationError`, `ErrNotFound`, `ErrBadRequest` and `*http.MaxBytesError` (also when wrapped) and hides unknown errors behind a 500.
- **Acceptance:** `Content-Type` is present in the recorded result (so it was set before `WriteHeader`); exact codes/messages; no leak of internal error text.
- <details><summary>Hint</summary>Put the most specific <code>errors.As</code>/<code>errors.Is</code> cases first in a <code>switch { case ...: }</code> and use a <code>map[string]errorBody</code> (or struct) so the outer key is <code>"error"</code>.</details>

**Exercise 2: List queries**
- **Task:** implement `ParseParams` (defaults, clamp, `*QueryError{Param}`), `Sort` (nulls last, ID tie-break) and generic `Paginate`.
- **Acceptance:** every table case in `listing_test.go`, including `limit=100000` -> 100, `offset` past the end -> `"items":[]`, and no aliasing of the input slice.
- <details><summary>Hint</summary>Clip with <code>start := min(offset, len)</code>, <code>end := min(start+limit, len)</code>. For the comparator use <code>cmp.Or(c, cmp.Compare(a.ID, b.ID))</code>, and return early for nil due dates *before* applying the descending flip.</details>

**Exercise 3: Create endpoint**
- **Task:** implement `NewCreateHandler` and `validate` for `POST /tasks` (types, `MemStore` and response helpers are given in the same file).
- **Acceptance:** 201 + `Location` + exact JSON; 400 for every malformed shape; 413 above 64 KiB; 422 with *all* broken fields; injected clock used for `created_at`; store errors never leak.
- <details><summary>Hint</summary>Wrap <code>r.Body</code> with <code>http.MaxBytesReader</code> first, decode with <code>DisallowUnknownFields</code>, then call <code>dec.Token()</code> and expect <code>io.EOF</code> to detect trailing data. Check the decode error with <code>errors.As(err, &amp;*http.MaxBytesError)</code>.</details>

## 7. Recap
- Pick the status by *outcome*: 201+`Location`, 204 without body, 400 for undecodable, 422 for rule violations, 404/405/413 as they occur.
- One envelope, one helper, no leaked internals.
- Validate everything and report all field errors at once; keep decoding errors separate from rules.
- Lists need defaults, a clamped `limit`, a deterministic sort and `[]` for empty.
- Reach for a custom type (`optional[T]`) when JSON semantics (`null` vs absent) matter; reach for an idempotency key when clients must retry a `POST`.

## 8. Self-check Questions
1. A client sends `{"title": 42}`, another sends `{"title": ""}`. Which status does each get, and why are they different?
2. Why must pagination sorting include a unique tie-breaker, and what would clients observe without it?
3. How does PATCH tell `{"due_date": null}` from `{}`, and why can't a plain `*string` do it?
4. Why is `DELETE` considered idempotent even though the second call returns 404?
5. What breaks if `w.WriteHeader(201)` is called before `w.Header().Set("Location", ...)`?

## 9. Further Reading
- [RFC 9110: HTTP Semantics](https://www.rfc-editor.org/rfc/rfc9110) (methods, status codes, idempotency)
- [Microsoft REST API Guidelines](https://github.com/microsoft/api-guidelines) and the [Idempotency-Key header draft](https://datatracker.ietf.org/doc/draft-ietf-httpapi-idempotency-key-header/)
- [OpenAPI Specification 3.1](https://spec.openapis.org/oas/v3.1.0)
- [encoding/json: Decoder.DisallowUnknownFields](https://pkg.go.dev/encoding/json#Decoder.DisallowUnknownFields)

---
**Next:** [04 Serialization](../04-serialization/README.md)
