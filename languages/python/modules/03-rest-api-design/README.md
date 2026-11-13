# 03. REST API Design in Python

> Estimated time: 2h | Difficulty: Intermediate | Prerequisites: [02 HTTP Server](../02-http-server/README.md)

## Learning Outcomes
After this module you can:
- [ ] Model resources and pick the right HTTP verb and status code for each operation
- [ ] Validate requests with Pydantic and return one consistent error envelope for every failure
- [ ] Implement pagination (with a clamped limit), filtering and sorting on a collection
- [ ] Version an API in the URL and make `PUT`, `DELETE` and `POST` (via `Idempotency-Key`) safe to retry
- [ ] Generate and customise OpenAPI documentation from the code

## 1. Concept Overview
REST models an API as *resources* (nouns such as `/tasks/7`) manipulated with a small set of verbs. Clients rely on the contract: which status code means what, what an error looks like, how lists are paged, and which requests are safe to retry.

| Verb | Meaning | Safe | Idempotent | Success |
|------|---------|------|-----------|---------|
| `GET` | read | yes | yes | 200 |
| `POST` | create / action | no | no (unless you add a key) | 201 + `Location` |
| `PUT` | replace the whole resource | no | yes | 200 |
| `PATCH` | change some fields | no | usually | 200 |
| `DELETE` | remove | no | yes (in effect) | 204 |

The shared rules for this course (see the [domain spec](../../../../framework/domain-and-projects.md)): errors are `{"error": {"code", "message", "fields"?}}`; lists are `{"items", "total", "limit", "offset"}` with `limit` default 20 and clamped to 100; malformed JSON and bad query values are `400`, well-formed but invalid bodies are `422`.

## 2. How Python Approaches It
- **Pydantic v2 models are the contract.** One class validates the request, documents the schema and serialises the response. `Field(...)`, `StringConstraints`, `Literal` and `StrEnum` express the rules declaratively; `field_validator` handles the rest.
- **FastAPI generates OpenAPI** at `/openapi.json` (and Swagger UI at `/docs`) from routes, models, `summary=`, `tags=` and `responses=`.
- **Return type annotations act as `response_model`:** only the declared fields are sent, so internal attributes cannot leak.
- **`model_fields_set` / `model_dump(exclude_unset=True)`** distinguish "absent" from "null", which `PATCH` needs.
- **Custom exception handlers** replace FastAPI's default `{"detail": ...}` (and its `422` for every validation problem) with the shared envelope.
- **Libraries (PROFILE.md):** `fastapi`, `pydantic`. Helper files shared by the examples: [m03_errors.py](examples/m03_errors.py) (envelope + handlers) and [m03_tasks.py](examples/m03_tasks.py) (validated Task models, in-memory store with filter/sort/paging).

## 3. Guided Examples
All commands run from `languages/python/`. Servers read `ADDR` (default `127.0.0.1:8080`) and stop on Ctrl-C/SIGTERM. Examples 2, 4 and 5 honour `FIXED_NOW=2030-01-01T09:00:00Z`, which freezes the clock so `created_at` is reproducible. The `curl` outputs below are real responses.

### 3.1 Resources, verbs and status codes
- **Goal:** map a resource to routes and status codes.
- **Run:** `python modules/03-rest-api-design/examples/01_resources.py`
- **Code:** [examples/01_resources.py](examples/01_resources.py)
- **Walkthrough:** the collection `/tasks` takes `POST` (201 + `Location`) and `GET`; the item `/tasks/{id}` takes `GET`, `PUT`, `PATCH`, `DELETE` (204, no body). `PATCH` uses `model_dump(exclude_unset=True)` so only sent fields change. Errors still use FastAPI's default `detail` body; the next example replaces it.

- **Expected output:**

  ```
  $ curl -si -X POST localhost:8080/tasks -H 'Content-Type: application/json' -d '{"title":"Write docs"}' | grep -iE '^(HTTP|location|\{)'
  HTTP/1.1 201 Created
  location: /tasks/1
  {"id":1,"title":"Write docs","done":false}
  $ curl -s -X PUT localhost:8080/tasks/1 -H 'Content-Type: application/json' -d '{"title":"Write better docs"}'
  {"id":1,"title":"Write better docs","done":false}
  $ curl -s -X PATCH localhost:8080/tasks/1 -H 'Content-Type: application/json' -d '{"done":true}'
  {"id":1,"title":"Write better docs","done":true}
  $ curl -si -X DELETE localhost:8080/tasks/1 | head -1
  HTTP/1.1 204 No Content
  $ curl -s localhost:8080/tasks/1
  {"detail":"task 1 not found"}
  ```

### 3.2 Validation and the error envelope
- **Goal:** one error shape for every failure.
- **Run:** `FIXED_NOW=2030-01-01T09:00:00Z python modules/03-rest-api-design/examples/02_validation_and_errors.py`
- **Code:** [examples/02_validation_and_errors.py](examples/02_validation_and_errors.py), rules in [m03_tasks.py](examples/m03_tasks.py), handlers in [m03_errors.py](examples/m03_errors.py)
- **Walkthrough:** `TaskCreate` trims and bounds the title, constrains tags, parses `due_date`, and forbids unknown fields (`extra="forbid"`). Four handlers produce the envelope: `ApiError` (our own), `HTTPException` (404/405/...), `RequestValidationError` (400 for broken JSON and bad path/query values, 422 for an invalid body, with `fields`), and `Exception` (500, details only in the log).

```python
@app.exception_handler(RequestValidationError)
async def on_validation_error(request, exc):
    if any(e["type"] == "json_invalid" for e in exc.errors()):
        return envelope(400, "bad_request", "malformed JSON body")
    ...
```

- **Expected output:**

  ```
  $ curl -si -X POST localhost:8080/tasks -H 'Content-Type: application/json' \
      -d '{"title":"  Write docs  ","tags":["docs","p1"],"due_date":"2030-01-31"}' | grep -iE '^(HTTP|location|\{)'
  HTTP/1.1 201 Created
  location: /tasks/1
  {"id":1,"title":"Write docs","status":"todo","due_date":"2030-01-31","tags":["docs","p1"],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}
  $ curl -s -X POST localhost:8080/tasks -H 'Content-Type: application/json' -d '{"title":"","tags":["Docs","docs"],"due_date":"tomorrow","bogus":1}'
  {"error":{"code":"validation_failed","message":"invalid request","fields":{"title":"String should have at least 1 character","due_date":"Input should be a valid date or datetime, input is too short","tags":"tags must be lowercase","bogus":"Extra inputs are not permitted"}}}
  $ curl -s -X POST localhost:8080/tasks -H 'Content-Type: application/json' -d '{"title":'
  {"error":{"code":"bad_request","message":"malformed JSON body"}}
  $ curl -s localhost:8080/tasks/abc
  {"error":{"code":"bad_request","message":"invalid request parameters","fields":{"task_id":"Input should be a valid integer, unable to parse string as an integer"}}}
  $ curl -s localhost:8080/tasks/99
  {"error":{"code":"not_found","message":"task 99 not found"}}
  $ curl -s localhost:8080/boom
  {"error":{"code":"internal","message":"internal server error"}}
  ```

### 3.3 Pagination, filtering and sorting
- **Goal:** a list endpoint with predictable paging.
- **Run:** `python modules/03-rest-api-design/examples/03_pagination_filtering_sorting.py` (seeds five tasks created one minute apart)
- **Code:** [examples/03_pagination_filtering_sorting.py](examples/03_pagination_filtering_sorting.py); filter/sort logic is `TaskStore.query` in [m03_tasks.py](examples/m03_tasks.py)
- **Walkthrough:** `Query(ge=1)` rejects `limit=0` (400), then `min(limit, MAX_LIMIT)` clamps oversized values and the *effective* limit is echoed back. `sort` is a `Literal`, so unknown values fail validation. `total` counts matches before paging. Ties are broken by `id` so pages are stable; tasks without a due date sort last.

```python
def list_tasks(limit: Annotated[int, Query(ge=1)] = DEFAULT_LIMIT) -> Page:
    limit = min(limit, MAX_LIMIT)  # clamp, then echo the effective limit
    ...
```

- **Expected output** (`titles` is a small shell helper that prints `total limit offset [titles]` from the JSON):

  ```
  $ titles() { python -c 'import json,sys; d=json.load(sys.stdin); print(d["total"], d["limit"], d["offset"], [t["title"] for t in d["items"]])'; }
  $ curl -s "localhost:8080/tasks?limit=1"
  {"items":[{"id":5,"title":"Plan sprint","status":"todo","due_date":"2030-01-10","tags":["p1"],"owner_id":null,"created_at":"2030-01-01T09:04:00Z"}],"total":5,"limit":1,"offset":0}
  $ curl -s "localhost:8080/tasks" | titles
  5 20 0 ['Plan sprint', 'Review PR', 'Ship release', 'Fix bug', 'Write docs']
  $ curl -s "localhost:8080/tasks?limit=2&offset=2" | titles
  5 2 2 ['Ship release', 'Fix bug']
  $ curl -s "localhost:8080/tasks?status=todo&sort=due_date" | titles
  3 20 0 ['Plan sprint', 'Write docs', 'Ship release']
  $ curl -s "localhost:8080/tasks?tag=p1&sort=created_at" | titles
  3 20 0 ['Fix bug', 'Ship release', 'Plan sprint']
  $ curl -s "localhost:8080/tasks?limit=0"
  {"error":{"code":"bad_request","message":"invalid request parameters","fields":{"limit":"Input should be greater than or equal to 1"}}}
  $ curl -s "localhost:8080/tasks?sort=title"
  {"error":{"code":"bad_request","message":"invalid request parameters","fields":{"sort":"Input should be 'created_at', '-created_at', 'due_date' or '-due_date'"}}}
  ```

  Requesting `limit=1000` returns `"limit":100`.

### 3.4 Versioning and idempotency
- **Goal:** evolve the URL space safely and make retries harmless.
- **Run:** `FIXED_NOW=2030-01-01T09:00:00Z python modules/03-rest-api-design/examples/04_versioning_and_idempotency.py`
- **Code:** [examples/04_versioning_and_idempotency.py](examples/04_versioning_and_idempotency.py)
- **Walkthrough:** an `APIRouter(prefix="/v1")` puts every route under a version; a breaking change becomes `/v2` next to it. `PUT` with the same body twice leaves the same state. For `POST`, the `Idempotency-Key` header maps to the stored response: same key + same body replays the first result; same key + different body is `409`. A `Lock` protects the check-then-create because plain `def` handlers run in threads.

```python
elif (previous := replays.get(idempotency_key)) is None:
    task = store.create(data)
    replays[idempotency_key] = (data.model_dump_json(), task)
```

- **Expected output:**

  ```
  $ curl -si -X POST localhost:8080/v1/tasks -H 'Content-Type: application/json' -H 'Idempotency-Key: abc-1' -d '{"title":"Pay invoice"}' | grep -iE '^(HTTP|location|idempotency-replayed|\{)'
  HTTP/1.1 201 Created
  location: /v1/tasks/1
  {"id":1,"title":"Pay invoice","status":"todo","due_date":null,"tags":[],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}
  ```

  The same command again returns the same task plus `idempotency-replayed: true` and no new task. With a different body:

  ```
  {"error":{"code":"conflict","message":"Idempotency-Key was already used with a different request"}}
  ```

  `DELETE /v1/tasks/1` answers `204` the first time and `404` (`"task 1 not found"`) the second; `GET /tasks/2` (no version prefix) is a `404` envelope.

### 3.5 The complete Task Tracker API and its OpenAPI document
- **Goal:** everything together, documented.
- **Run:** `FIXED_NOW=2030-01-01T09:00:00Z python modules/03-rest-api-design/examples/05_tasks_api.py`
- **Code:** [examples/05_tasks_api.py](examples/05_tasks_api.py)
- **Walkthrough:** `create_app(clock)` is a factory (tests inject a clock). `TaskPatch` allows an explicit `null` only for `due_date`. `responses=ERRORS` on the router documents the envelope for 400/404/422 and replaces FastAPI's default `HTTPValidationError` schema (router-level, so it is listed on every operation; refine per route when it matters). Browse `/docs` or `/openapi.json`.

- **Expected output:**

  ```
  $ curl -s -X POST localhost:8080/v1/tasks -H 'Content-Type: application/json' -d '{"title":"Write docs","due_date":"2030-01-31","tags":["docs"]}'
  {"id":1,"title":"Write docs","status":"todo","due_date":"2030-01-31","tags":["docs"],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}
  $ curl -s -X PATCH localhost:8080/v1/tasks/1 -H 'Content-Type: application/json' -d '{"status":"in_progress"}'
  {"id":1,"title":"Write docs","status":"in_progress","due_date":"2030-01-31","tags":["docs"],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}
  $ curl -s -X PATCH localhost:8080/v1/tasks/1 -H 'Content-Type: application/json' -d '{"due_date":null}'
  {"id":1,"title":"Write docs","status":"in_progress","due_date":null,"tags":["docs"],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}
  $ curl -s -X PATCH localhost:8080/v1/tasks/1 -H 'Content-Type: application/json' -d '{"title":null,"owner_id":5}'
  {"error":{"code":"validation_failed","message":"invalid request","fields":{"title":"must not be null","owner_id":"Extra inputs are not permitted"}}}
  $ curl -s -X PATCH localhost:8080/v1/tasks/9 -H 'Content-Type: application/json' -d '{"status":"done"}'
  {"error":{"code":"not_found","message":"task 9 not found"}}
  $ curl -s localhost:8080/openapi.json | python -c 'import json,sys; s=json.load(sys.stdin); [print(m.upper(), p, sorted(o["responses"])) for p,ops in s["paths"].items() for m,o in ops.items()]; print(sorted(s["components"]["schemas"]))'
  GET /healthz ['200']
  POST /v1/tasks ['201', '400', '404', '422']
  GET /v1/tasks ['200', '400', '404', '422']
  GET /v1/tasks/{task_id} ['200', '400', '404', '422']
  PUT /v1/tasks/{task_id} ['200', '400', '404', '422']
  PATCH /v1/tasks/{task_id} ['200', '400', '404', '422']
  DELETE /v1/tasks/{task_id} ['204', '400', '404', '422']
  ['ErrorBody', 'ErrorEnvelope', 'Status', 'Task', 'TaskCreate', 'TaskPage', 'TaskPatch']
  ```

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| Clients get `{"detail": [...]}` for validation errors but `{"error": ...}` for the rest | FastAPI's default handlers return `detail` and always use `422` | register handlers for `RequestValidationError` and `HTTPException` (and `Exception`) that build the envelope |
| Returning the stored/ORM object leaks internal fields (password hash, owner flags) | no `response_model`, so everything is serialised | annotate the return type with a dedicated response model |
| `PATCH` cannot tell "absent" from `null` | both arrive as `None` in a model with defaults | use `model_dump(exclude_unset=True)` / `model_fields_set` and reject `null` where it is not allowed |
| `limit=100000` loads the whole table; or it is rejected with a surprising 422 | no upper bound, or `le=100` rejects instead of clamping | `Query(ge=1)` then `min(limit, MAX_LIMIT)`, and echo the effective limit |
| Unstable pages (duplicates or gaps between requests) | sorting by a non-unique key | add a tiebreaker (`id`) to every sort |
| Unknown JSON fields silently ignored, typos never noticed | Pydantic ignores extra keys by default | `ConfigDict(extra="forbid")` on input models |
| Mutable module-level `tasks = []` shared across tests and workers | convenient for demos | a `create_app()` factory with the store created inside; use a lock or a database for shared mutable state |
| Idempotency map checked and written without a lock | `def` handlers run in a threadpool, so two requests can interleave | guard check-then-create with `threading.Lock` (or a database unique constraint) |
| `str` where `StrEnum`/`Literal` belongs, so `status=finished` reaches the store | typing the field as plain text is easy | use an enum or `Literal`; invalid values then fail validation automatically |
| `201` without `Location`, `204` with a body | status code and headers are separate decisions | follow the verb table; `204` handlers return `None` |

## 5. Coming From Other Languages
- **TypeScript (Fastify/zod):** a Pydantic model is the zod schema *and* the TS type in one; OpenAPI comes from the code rather than a plugin.
- **Java/C# (Spring, ASP.NET):** `@Valid`/data annotations map to `Field` constraints; `ProblemDetails` (RFC 9457) is the C# analogue of the envelope here, which the Python track keeps identical to Go/Rust/TypeScript.
- **Go:** validation is explicit code in Go; in FastAPI it is declared on the types, but you still write the error mapping yourself.
- **Ruby/Django REST:** serializers correspond to Pydantic models; pagination classes correspond to the `Page` model plus `Query` bounds.

## 6. Exercises
Tests use FastAPI's `TestClient` (no network). Solutions are in `solutions/`.

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | [Status codes](exercises/ex1_status_codes/m03_resources.py) | Easy | verbs, 201/204/404, `Location` | `pytest modules/03-rest-api-design/exercises/ex1_status_codes` |
| 2 | [Validation](exercises/ex2_validation/m03_validation.py) | Medium | Pydantic constraints, error envelope | `pytest modules/03-rest-api-design/exercises/ex2_validation` |
| 3 | [Listing](exercises/ex3_listing/m03_listing.py) | Stretch | filter, sort, clamp, idempotent POST | `pytest modules/03-rest-api-design/exercises/ex3_listing` |

**Exercise 1: Status codes**
- **Task:** implement a `/notes` resource (`POST`, `GET` list, `GET`, `PUT`, `DELETE`) with the right status codes, `Location` header, empty `204` body and a `404` envelope.
- **Acceptance:** all tests in `test_m03_resources.py` pass, including "ids are never reused" and "second `DELETE` is `404`".
- <details><summary>Hint</summary>Use <code>status_code=201</code> / <code>204</code> in the decorator, a <code>Response</code> parameter for <code>Location</code>, and raise the provided <code>NotFound</code> from one shared lookup helper.</details>

**Exercise 2: Validation**
- **Task:** make `TaskIn` strict and implement `install_error_handlers` so every failure (bad JSON, bad query, invalid body, 404/405, crash) uses the envelope with the right status.
- **Acceptance:** all tests in `test_m03_validation.py` pass, including nested field names (`tags.0`) and no leaked exception text on `500`.
- <details><summary>Hint</summary>Look at <code>error["type"]</code> and <code>error["loc"][0]</code> in <code>RequestValidationError.errors()</code>; the first <code>loc</code> element tells you <code>body</code> vs <code>query</code>.</details>

**Exercise 3: Listing**
- **Task:** implement `GET /tasks` (filters, sort with undated tasks last, clamped `limit`, `offset`, `total`) and `Idempotency-Key` support on `POST /tasks`.
- **Acceptance:** all tests in `test_m03_listing.py` pass: stable ties, replay header, `409` on a changed body.
- <details><summary>Hint</summary>Filter first, then sort with a key tuple such as <code>(due_date is None, sign * ordinal, id)</code>, then slice; remember <code>total</code> is the length before slicing.</details>

## 7. Recap
- Resources are nouns, verbs carry the meaning, status codes carry the outcome; `201` + `Location` on create, `204` on delete.
- Validate at the edge with Pydantic; reject unknown fields; use `400` for malformed requests and `422` for invalid data.
- One error envelope, installed once with exception handlers (including a fixed `500` that hides details).
- Lists: filters, a sort whitelist with a tiebreaker, `limit` clamped, `total` before paging.
- Retries are safe when `PUT`/`DELETE` are idempotent and `POST` accepts an `Idempotency-Key`; let FastAPI publish OpenAPI.

When to reach for what: whole-resource update -> `PUT`; partial -> `PATCH` with `exclude_unset`; client may retry a create -> `Idempotency-Key`.

## 8. Self-check Questions
1. Why is a malformed JSON body `400` but a body with `"title": ""` `422`, and how does the handler tell them apart?
2. What does it mean that `PUT` and `DELETE` are idempotent, and why is a second `DELETE` returning `404` still considered idempotent?
3. Why must every sort have a unique tiebreaker for pagination to be correct?
4. How does `PATCH` distinguish `{"due_date": null}` from an absent `due_date`?
5. What can go wrong if the idempotency store is not protected against concurrent requests?

## 9. Further Reading
- [FastAPI: handling errors](https://fastapi.tiangolo.com/tutorial/handling-errors/) and [response model](https://fastapi.tiangolo.com/tutorial/response-model/)
- [Pydantic: validators](https://docs.pydantic.dev/latest/concepts/validators/) and [fields and constraints](https://docs.pydantic.dev/latest/concepts/fields/)
- [RFC 9110 (HTTP semantics)](https://www.rfc-editor.org/rfc/rfc9110) for verbs and status codes; the [OpenAPI specification](https://spec.openapis.org/oas/latest.html)

---
**Next:** [04 Serialization](../04-serialization/README.md)
