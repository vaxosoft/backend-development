# 03. REST API Design in Rust

> Estimated time: 2h | Difficulty: Intermediate | Prerequisites: [01 Language Fundamentals](../01-language-fundamentals/README.md), [02 HTTP Server](../02-http-server/README.md)

## Learning Outcomes
After this module you can:
- [ ] Model resources and choose the right HTTP verb and status code for each operation
- [ ] Validate request data and report every problem in one consistent JSON error envelope
- [ ] Implement pagination, filtering and sorting with safe defaults and clamping
- [ ] Explain and implement idempotency (`PUT`, `DELETE`, `Idempotency-Key`) and URL versioning
- [ ] Describe an API with an OpenAPI document

## 1. Concept Overview
REST design is a contract: **nouns** for resources (`/tasks`, `/tasks/{id}`), **verbs** for what you do to them, **status codes** for what happened, and one predictable **error shape** so clients can handle failures generically.

| Operation | Request | Success | Typical errors |
|-----------|---------|---------|----------------|
| create | `POST /tasks` | `201` + `Location` | `400` bad JSON, `422` invalid data |
| read | `GET /tasks/{id}` | `200` | `404` |
| replace | `PUT /tasks/{id}` (whole resource) | `200` | `404`, `422` |
| update | `PATCH /tasks/{id}` (absent = unchanged) | `200` | `404`, `422` |
| delete | `DELETE /tasks/{id}` | `204` | `404` |
| list | `GET /tasks?status=&tag=&sort=&limit=&offset=` | `200` `{items,total,limit,offset}` | `400` bad parameter |

`400` means "I could not understand you" (malformed JSON, `limit=abc`); `422` means "I understood you but the data is invalid" (title too long) and carries per-field messages. **Idempotent** operations can be repeated safely: `GET`, `PUT`, `DELETE` by definition, `POST` only with an idempotency key. See [the shared conventions](../../../../framework/domain-and-projects.md) for the full table used across all languages.

## 2. How Rust Approaches It
- **One typed error:** an `ApiError` enum implements `IntoResponse`, so handlers return `Result<T, ApiError>` and use `?`. The status code and JSON body live in one place ([src/error.rs](src/error.rs)).
- **Rejections become envelopes:** axum's built-in extractor errors are plain text. A tiny wrapper (`#[derive(FromRequest)] #[from_request(via(axum::Json), rejection(ApiError))] struct ApiJson<T>`) maps them to the envelope. `ApiQuery` and `ApiPath` do the same.
- **Validation is explicit code, not derive magic:** deserialize into a struct of `Option` fields, then a plain function checks every rule and returns all violations ([src/tasks.rs](src/tasks.rs)). No validator crate; the rules are easy to test.
- **PATCH semantics via the type system:** `Option<Option<T>>` separates *absent* from *`null`* from *value* with a 5-line `deserialize_with`.
- **Versioning** with `Router::nest("/v1", ..)`; **documentation** with a hand-written `openapi.json` served by a handler (there is no stable code-first generator in the pinned set).
- Libraries: `axum`, `serde`/`serde_json`, `thiserror` only.

## 3. Guided Examples
Commands run from `languages/rust/`; servers read `ADDR` (default `127.0.0.1:8080`) and stop on Ctrl-C. The shared code is in [src/](src/). Outputs omit `date` and `content-length` headers. JSON keys appear in alphabetical order inside the error envelope (a `serde_json` detail); clients must not depend on key order.

### 3.1 The error envelope
- **Goal:** every failure, whatever its origin, has one JSON shape.
- **Run:** `cargo run -p m03-rest-api-design --example error_envelope`
- **Code:** [examples/error_envelope.rs](examples/error_envelope.rs), [src/error.rs](src/error.rs)
```rust
impl IntoResponse for ApiError {
    fn into_response(self) -> Response {
        // Internal details go to the log, never to the client.
        let mut error = json!({ "code": self.code(), "message": message });
        if let ApiError::Validation { fields, .. } = &self { error["fields"] = json!(fields); }
        (self.status(), Json(json!({ "error": error }))).into_response()
    }
}
async fn get_note(ApiPath(id): ApiPath<u32>) -> Result<Json<Value>, ApiError> { /* ... */ }
```
- **Walkthrough:** `ApiError::status()` / `code()` are an exhaustive `match`, so a new variant cannot be forgotten. `From<JsonRejection>` maps a syntax error to `400`, wrong-shaped JSON to `422`, and an oversized body to `413`. `.fallback(not_found_fallback)` gives unknown routes the same envelope.
- **Expected output:**
  ```
  $ curl -si localhost:8080/notes/9
  HTTP/1.1 404 Not Found
  content-type: application/json

  {"error":{"code":"not_found","message":"note 9 does not exist"}}
  $ curl -s localhost:8080/notes/abc
  {"error":{"code":"bad_request","message":"Invalid URL: Cannot parse `abc` to a `u32`"}}
  $ curl -s -H 'Content-Type: application/json' -d '{"text":"  "}' localhost:8080/notes
  {"error":{"code":"validation_failed","fields":{"text":"must not be empty"},"message":"invalid note"}}
  $ curl -s -H 'Content-Type: application/json' -d '{"text":' localhost:8080/notes
  {"error":{"code":"bad_request","message":"malformed JSON: Failed to parse the request body as JSON: text: EOF while parsing a value at line 1 column 8"}}
  $ curl -s -H 'Content-Type: application/json' -d '{"text":5}' localhost:8080/notes
  {"error":{"code":"validation_failed","fields":{"body":"Failed to deserialize the JSON body into the target type: text: invalid type: integer `5`, expected a string at line 1 column 9"},"message":"invalid request body"}}
  $ curl -s localhost:8080/conflict
  {"error":{"code":"conflict","message":"slug already taken"}}
  $ curl -s localhost:8080/boom
  {"error":{"code":"internal","message":"internal server error"}}
  $ curl -s localhost:8080/nope
  {"error":{"code":"not_found","message":"no such route"}}
  ```
  The server log (stderr) shows `internal error: connection pool exhausted at db.rs:42` for `/boom`; the client never sees it. A wrong-method request (`DELETE /notes/1`) still gets axum's empty `405` with an `allow` header; map it with `Router::method_not_allowed_fallback` if you need the envelope.

### 3.2 Validation
- **Goal:** collect all field errors, normalise input, answer `422`.
- **Run:** `cargo run -p m03-rest-api-design --example validation`
- **Code:** [examples/validation.rs](examples/validation.rs), `validate_task` in [src/tasks.rs](src/tasks.rs)
```rust
let title = match input.title.as_deref() {
    Some(raw) => collect(&mut fields, "title", check_title(raw)),   // trims, 1-200 chars
    None => collect(&mut fields, "title", Err("is required")),
};
// ... status, due_date, tags, owner_id the same way ...
match (title, status, due_date, tags, owner_id) {
    (Some(title), Some(status), Some(due_date), Some(tags), Some(owner_id)) => Ok(NewTask { .. }),
    _ => Err(ApiError::validation("invalid task", fields)),
}
```
- **Walkthrough:** `TaskInput` uses `Option` for every field so *we* decide what "missing" means and report it per field. The checks are small pure functions (`check_title`, `check_tags`, `is_valid_date` with leap years) that return a message or a normalised value. The endpoint is a dry run that echoes the normalised task.
- **Expected output:**
  ```
  $ curl -s -H 'Content-Type: application/json' -d '{"title":"  Write docs ","tags":["docs","p1"],"due_date":"2032-02-29","owner_id":4}' localhost:8080/tasks/validate
  {"due_date":"2032-02-29","owner_id":4,"status":"todo","tags":["docs","p1"],"title":"Write docs"}
  $ curl -s -H 'Content-Type: application/json' -d '{}' localhost:8080/tasks/validate
  {"error":{"code":"validation_failed","fields":{"title":"is required"},"message":"invalid task"}}
  $ curl -s -H 'Content-Type: application/json' -d '{"title":"","status":"finished","due_date":"2030-13-01","tags":["Docs","docs"],"owner_id":0}' localhost:8080/tasks/validate
  {"error":{"code":"validation_failed","fields":{"due_date":"must be a calendar date in YYYY-MM-DD format","owner_id":"must be a positive integer","status":"must be one of todo, in_progress, done","tags":"tags must be lowercase","title":"must be 1-200 characters"},"message":"invalid task"}}
  $ curl -s -H 'Content-Type: application/json' -d '{"title":"x","due_date":"2030-02-29"}' localhost:8080/tasks/validate
  {"error":{"code":"validation_failed","fields":{"due_date":"must be a calendar date in YYYY-MM-DD format"},"message":"invalid task"}}
  ```

### 3.3 Pagination, filtering, sorting
- **Goal:** a list endpoint with safe defaults.
- **Run:** `cargo run -p m03-rest-api-design --example pagination`
- **Code:** [examples/pagination.rs](examples/pagination.rs), `list_tasks` in [src/list.rs](src/list.rs)
```rust
let limit = query.limit.unwrap_or(DEFAULT_LIMIT).clamp(1, MAX_LIMIT);   // 20 by default, at most 100
let total = matches.len();                                               // before slicing
let items = matches.into_iter().skip(offset).take(limit).cloned().collect();
```
- **Walkthrough:** five seeded tasks. `ApiQuery<ListQuery>` rejects `limit=abc` and `offset=-1` with `400` before the handler runs. `status` and `sort` are validated by hand (`400` with a helpful message). `sort=due_date` puts tasks without a due date last in both directions. Ties break on `id`, so pages are stable.
- **Expected output:**
  ```
  $ curl -s 'localhost:8080/tasks?limit=2'
  {"items":[{"id":5,"title":"Update deps","status":"todo","due_date":"2030-01-10","tags":["chore","p1"],"owner_id":null,"created_at":"2030-01-05T09:00:00Z"},{"id":4,"title":"Refactor router","status":"todo","due_date":null,"tags":["tech-debt"],"owner_id":null,"created_at":"2030-01-04T09:00:00Z"}],"total":5,"limit":2,"offset":0}
  $ curl -s 'localhost:8080/tasks?limit=1000&offset=4'
  {"items":[{"id":1,"title":"Write docs","status":"todo","due_date":"2030-02-01","tags":["docs","p1"],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}],"total":5,"limit":100,"offset":4}
  $ curl -s 'localhost:8080/tasks?sort=title'
  {"error":{"code":"bad_request","message":"sort must be one of created_at, -created_at, due_date, -due_date"}}
  $ curl -s 'localhost:8080/tasks?limit=abc'
  {"error":{"code":"bad_request","message":"Failed to deserialize query string: limit: invalid digit found in string"}}
  $ curl -s 'localhost:8080/tasks?status=nope'
  {"error":{"code":"bad_request","message":"status must be one of todo, in_progress, done"}}
  ```
  Try `?status=todo&sort=due_date` and `?tag=p1&sort=-due_date` yourself: ids come back as `5,1,4` and `1,2,5`.

### 3.4 Idempotency keys
- **Goal:** make `POST` safe to retry.
- **Run:** `cargo run -p m03-rest-api-design --example idempotency`
- **Code:** [examples/idempotency.rs](examples/idempotency.rs)
```rust
if let Some((original, task)) = app.replays.get(&key) {
    if *original != fingerprint { return Err(ApiError::Conflict("...different request body".into())); }
    return Ok(created(task, true));   // replay: same 201, plus `idempotent-replayed: true`
}
```
- **Walkthrough:** the first request with a key creates the task and stores `(body fingerprint, task)`. A retry returns the stored result without creating a second task. Reusing a key for a *different* body is a client bug: `409`. `PUT` and `DELETE` need no key: repeating them leaves the same state. (A production store would expire keys and survive restarts.)
- **Expected output:**
  ```
  $ curl -si -H 'Content-Type: application/json' -H 'Idempotency-Key: k1' -d '{"title":"Pay invoice"}' localhost:8080/tasks
  HTTP/1.1 201 Created
  content-type: application/json
  location: /tasks/1

  {"id":1,"title":"Pay invoice","status":"todo","due_date":null,"tags":[],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}
  $ (same command again)
  HTTP/1.1 201 Created
  content-type: application/json
  location: /tasks/1
  idempotent-replayed: true

  {"id":1,"title":"Pay invoice","status":"todo","due_date":null,"tags":[],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}
  $ curl -si -H 'Content-Type: application/json' -H 'Idempotency-Key: k1' -d '{"title":"Something else"}' localhost:8080/tasks
  HTTP/1.1 409 Conflict
  content-type: application/json

  {"error":{"code":"conflict","message":"Idempotency-Key was already used with a different request body"}}
  $ curl -s localhost:8080/tasks
  {"total":1}
  ```

### 3.5 Task Tracker API with versioning and OpenAPI
- **Goal:** the whole resource: create, read, replace, patch, delete, list, under `/v1`, documented.
- **Run:** `cargo run -p m03-rest-api-design --example tasks_api`
- **Code:** [examples/tasks_api.rs](examples/tasks_api.rs), [examples/tasks_api.openapi.json](examples/tasks_api.openapi.json)
```rust
let v1 = Router::new()
    .route("/tasks", get(list).post(create))
    .route("/tasks/{id}", get(read).put(replace).patch(patch).delete(delete));
Router::new().nest("/v1", v1).route("/openapi.json", get(openapi)).with_state(Db::default())
```
- **Walkthrough:** each handler is `extract -> validate -> use the store -> map None to 404`. `PatchInput.due_date` is `Option<Option<String>>`: `{"due_date":null}` clears it, omitting it leaves it alone. `Location` is the versioned URL. A breaking change would ship as `/v2` beside `/v1`; unversioned `/tasks` is a `404`.
- **Expected output:**
  ```
  $ curl -si -H 'Content-Type: application/json' -d '{"title":"Write docs","tags":["docs"],"due_date":"2030-01-31"}' localhost:8080/v1/tasks
  HTTP/1.1 201 Created
  content-type: application/json
  location: /v1/tasks/1

  {"id":1,"title":"Write docs","status":"todo","due_date":"2030-01-31","tags":["docs"],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}
  $ curl -s -X PATCH -H 'Content-Type: application/json' -d '{"status":"done"}' localhost:8080/v1/tasks/1
  {"id":1,"title":"Write docs","status":"done","due_date":"2030-01-31","tags":["docs"],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}
  $ curl -s -X PATCH -H 'Content-Type: application/json' -d '{"due_date":null}' localhost:8080/v1/tasks/1
  {"id":1,"title":"Write docs","status":"done","due_date":null,"tags":["docs"],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}
  $ curl -s -X PATCH -H 'Content-Type: application/json' -d '{"status":"nope","tags":["A"]}' localhost:8080/v1/tasks/1
  {"error":{"code":"validation_failed","fields":{"status":"must be one of todo, in_progress, done","tags":"tags must be lowercase"},"message":"invalid task"}}
  $ curl -s -X PUT -H 'Content-Type: application/json' -d '{"title":"x"}' localhost:8080/v1/tasks/99
  {"error":{"code":"not_found","message":"task 99 not found"}}
  $ curl -s 'localhost:8080/v1/tasks?status=done'
  {"items":[{"id":1,"title":"Write docs","status":"done","due_date":null,"tags":["docs"],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}],"total":1,"limit":20,"offset":0}
  $ curl -si -X DELETE localhost:8080/v1/tasks/1 | head -1
  HTTP/1.1 204 No Content
  $ curl -s localhost:8080/tasks
  {"error":{"code":"not_found","message":"no such route"}}
  $ curl -s localhost:8080/openapi.json | head -4
  {
    "openapi": "3.1.0",
    "info": { "title": "Task Tracker API", "version": "1.0.0" },
    "servers": [{ "url": "/v1" }],
  ```

## 4. Common Pitfalls
| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| Clients get plain-text errors for bad JSON, path or query | axum's built-in rejections are not your envelope | wrap `Json`/`Query`/`Path` with `#[from_request(via(..), rejection(ApiError))]` |
| `422` vs `400` mix-ups | axum returns `422` for wrong-shape JSON and `400` for syntax errors; both come from `Json` | map explicitly in `From<JsonRejection>`: syntax -> `400`, data -> `422` (per-field messages need your own validation) |
| Required field reported as "missing field `title`" with no field name | `title: String` makes serde fail the whole body | deserialize `Option<String>` and report `fields.title = "is required"` yourself |
| PATCH cannot clear a field | `Option<T>` merges absent and `null` | `Option<Option<T>>` + `#[serde(default, deserialize_with = ..)]` |
| `Query<Vec<String>>` for `?tag=a&tag=b` | no repeated-key lists in `serde_urlencoded` | one param (`tag=a`) or a comma separated value |
| `limit` as `i32`, accepting `-5`, or unbounded | defaults left to chance | use `usize` (negatives are rejected), `clamp(1, MAX)`, document the maximum |
| Leaking `err.to_string()` of a database or IO error in a `500` | convenient | `ApiError::Internal(detail)` logs it, the body says only "internal server error" |
| Offset pagination with no stable order | ties in the sort key reorder between requests | always add a unique tiebreaker (`id`) |
| `String.len()` as "characters" in validation | `len()` counts bytes | use `.chars().count()` (or graphemes if you must) |
| Changing a response shape under `/v1` | clients depend on it | add fields compatibly; breaking changes go to `/v2` |

## 5. Coming From Other Languages
- **Go:** instead of `json.NewDecoder(r.Body).Decode(&v)` plus manual `http.Error`, the extractor decodes and the `Result` carries the failure; `IntoResponse` replaces a shared `writeError` helper.
- **C#:** idiomatic ASP.NET returns `ProblemDetails` (RFC 9457) and uses `[ApiController]` model validation. Here the envelope is `{"error":{code,message,fields}}` and validation is an explicit function; the status codes are the same.
- **Python (FastAPI/Pydantic):** Pydantic validates during parsing; in Rust serde checks *shape* and your code checks *rules*. `Option<Option<T>>` plays the role of Pydantic's "unset" sentinel.
- **Java/Spring:** `@Valid` + `@ControllerAdvice` correspond to validation functions + `impl IntoResponse for ApiError`.

## 6. Exercises
| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | [API error envelope](exercises/ex1-api-error) | Easy | `IntoResponse`, status mapping, `Retry-After`, rejection mapping | `cargo test -p m03-ex1-api-error` |
| 2 | [Validate users + PATCH](exercises/ex2-validate-user) | Medium | field validation, absent vs `null`, partial updates | `cargo test -p m03-ex2-validate-user` |
| 3 | [List users](exercises/ex3-list-users) | Stretch | search, sort, pagination, clamping, stable ordering | `cargo test -p m03-ex3-list-users` |

**1. API error envelope.** *Task:* implement `status`, `code`, `IntoResponse` and `From<JsonRejection>` for `ApiError` (table in the crate docs): `Validation` adds `fields`, `RateLimited` adds `Retry-After`, `Internal` never exposes its detail. *Acceptance:* `tests/api_error.rs`, run in-process with `oneshot`. *Hint:* <details><summary>nudge</summary>build the body with `serde_json::json!`, add `fields` only for `Validation`, and set the header on the `Response` returned by `(status, Json(..)).into_response()`.</details>

**2. Validate users + PATCH.** *Task:* implement `validate_user`, `validate_user_patch` and `apply_patch` (rules in the crate docs). *Acceptance:* `tests/validate_user.rs`: all violations at once, name length in characters, `null` rejected, an empty patch rejected. *Hint:* <details><summary>nudge</summary>`input.name.as_deref().ok_or("is required").and_then(check_name)` turns an `Option` into a `Result`; match `None / Some(None) / Some(Some(v))` for patches.</details>

**3. List users.** *Task:* implement `list_users` (`q`, `sort`, `limit`, `offset`, `total`). *Acceptance:* `tests/list_users.rs` (pure function plus HTTP through `router`). *Hint:* <details><summary>nudge</summary>filter first, sort the references with `sort_by` and `.then(a.id.cmp(&b.id))`, take `total` before `skip/take`.</details>

## 7. Recap
- One `ApiError` type, one JSON envelope, one place that decides status codes.
- `400` = unparseable, `422` = parseable but invalid (with per-field messages), `404`/`409` for resource state.
- Validate in plain functions on `Option` fields; report every problem together; normalise (trim) on the way in.
- List endpoints: defaults, clamping, stable sort with a tiebreaker, `total` before slicing.
- Reach for `Option<Option<T>>` when PATCH must distinguish absent from `null`, and for idempotency keys on retried `POST`s.

## 8. Self-check Questions
1. A client sends `{"title": 5}` and another sends `{"title": "   "}`. Which status does each get, and why do they differ?
2. Why does a `PATCH` handler need three states for `due_date`, and how does `Option<Option<String>>` encode them?
3. What breaks if two tasks have the same `created_at` and the list sort has no tiebreaker?
4. Why should `DELETE /tasks/1` called twice return `204` then `404`, and is it still idempotent?
5. What is the risk of returning `err.to_string()` for an unexpected error, and what does `ApiError::Internal` do instead?

## 9. Further Reading
- [RFC 9110: HTTP semantics](https://www.rfc-editor.org/rfc/rfc9110) (methods, idempotency, status codes) and [RFC 9457: problem details](https://www.rfc-editor.org/rfc/rfc9457) (the C# track's format)
- [OpenAPI Specification 3.1](https://spec.openapis.org/oas/v3.1.0)
- [axum: customizing extractor errors](https://github.com/tokio-rs/axum/tree/main/examples/customize-extractor-error)

---
**Next:** [04 Serialization](../04-serialization/README.md)
