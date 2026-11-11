# 03. REST API Design in C#

> Estimated time: 2h | Difficulty: Intermediate | Prerequisites: [01](../01-language-fundamentals/README.md), [02 HTTP Server](../02-http-server/README.md)

## Learning Outcomes
After this module you can:
- [ ] Model resources as nouns and pick the right verb and status code (200/201/204/400/404/409/415/422)
- [ ] Validate requests declaratively and answer with a consistent `application/problem+json` error body
- [ ] Keep entities and wire types apart (DTOs) so clients cannot over-post
- [ ] Paginate, filter and sort collections with bounded, allow-listed parameters
- [ ] Version an API, make `POST` safe to retry with an idempotency key, and publish an OpenAPI document

## 1. Concept Overview
REST over HTTP means: URLs name **resources** (`/tasks/42`), verbs say what to do, status codes say what happened, and
errors have one predictable shape so clients can handle them generically.

| Verb | Meaning | Success | Safe | Idempotent |
|------|---------|---------|------|------------|
| GET | read | 200 | yes | yes |
| POST | create / run an action | 201 + `Location` | no | no (unless you add an idempotency key) |
| PUT | replace the whole resource | 200 | no | yes |
| PATCH | change some fields | 200 | no | usually |
| DELETE | remove | 204 | no | yes (the *effect*; a second call may be 404) |

Failures: **400** malformed request (bad JSON, unparsable query value), **404** unknown resource, **409** conflict with current state,
**415** wrong `Content-Type`, **422** well-formed but breaks the rules (validation), **500** never leaks details.

## 2. How C# Approaches It
- **Wire format** is the shared Task Tracker contract: `snake_case` JSON (`SnakeCaseLower` naming policy), enums as strings, `DateOnly` as `yyyy-MM-dd`.
- **Errors:** other tracks use a custom envelope; idiomatic ASP.NET Core uses **ProblemDetails** (RFC 9457,
  `application/problem+json`) and `ValidationProblemDetails` (`errors`: field -> messages) with the *same status codes*.
  Lists use `{"items","total","limit","offset"}` in every track.
- **Results:** return `TypedResults.*` and declare `Results<Created<T>, ProblemHttpResult>`; the compiler checks the
  union and OpenAPI learns the possible responses.
- **Validation:** DataAnnotations (`[Required]`, custom `ValidationAttribute`) + `builder.Services.AddValidation()` (ASP.NET Core 10) checks minimal-API parameters before your handler runs.
- **OpenAPI:** `AddOpenApi()` + `MapOpenApi()` (package `Microsoft.AspNetCore.OpenApi`, version pinned centrally).
- **Idiomatic vs not:** records for `TaskInput`/`TaskItem`, a `Problems` helper instead of hand-written JSON; avoid exposing the storage entity and avoid `object` results.

## 3. Guided Examples
One web project (`ADDR` env var sets the listen address, default `127.0.0.1:8080`). Shared types live in [examples/Shared/](examples/Shared/).
In the output below `trace_id` (a per-request value ProblemDetails adds) is stripped with
`sed -E 's/,"trace_id":"[^"]*"//'`; the `Date`/`Server` headers are omitted.

### 3.1 Resources, verbs and status codes
- **Goal:** CRUD on `/tasks` with the right codes and a DTO boundary
- **Run:** `dotnet run --project modules/03-rest-api-design/examples -- 01-resources`
- **Code:** [01_Resources.cs](examples/01_Resources.cs), models in [Shared/TaskModels.cs](examples/Shared/TaskModels.cs)
  ```csharp
  static Results<Created<TaskItem>, ProblemHttpResult> Create(TaskInput input, TaskRepository repo) { ... }
  tasks.MapPost("/", Create);  tasks.MapGet("/{id:int}", Get);  tasks.MapPut(...);  tasks.MapDelete(...);
  ```
- **Walkthrough:** `TaskInput` has no `Id`/`CreatedAt`, so `{"id":99,...}` in a request is ignored (no over-posting).
  A duplicate title is a `409`; a blank one a `422`; unknown ids are `404` problem documents (`UseStatusCodePages` also turns bare framework 4xx into problem documents).
- **Expected output:**
  ```
  $ curl -si -X POST -H 'Content-Type: application/json' -d '{"id":99,"title":"Write docs","tags":["docs"],"created_at":"1999-01-01T00:00:00Z"}' localhost:8080/tasks
  HTTP/1.1 201 Created
  Content-Type: application/json; charset=utf-8
  Location: /tasks/1

  {"id":1,"title":"Write docs","status":"todo","due_date":null,"tags":["docs"],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}
  $ ... -d '{"title":"Write docs"}' localhost:8080/tasks
  HTTP/1.1 409 Conflict
  {"type":"https://tools.ietf.org/html/rfc9110#section-15.5.10","title":"Conflict","status":409,"detail":"a task with this title already exists"}
  $ ... -d '{"title":" "}' localhost:8080/tasks
  HTTP/1.1 422 Unprocessable Entity
  {"type":"https://www.rfc-editor.org/rfc/rfc9110#section-15.5.21","title":"One or more validation errors occurred.","status":422,"errors":{"title":["is required"]}}
  $ ... -d '{"title":' localhost:8080/tasks                  -> 400 Bad Request
  $ curl -X POST -H 'Content-Type: text/plain' -d x ...      -> 415 Unsupported Media Type
  $ curl -s localhost:8080/tasks/7
  {"type":"https://tools.ietf.org/html/rfc9110#section-15.5.5","title":"Not Found","status":404,"detail":"task 7 does not exist"}
  $ curl -si -X PUT ... -d '{"title":"Write better docs","status":"in_progress"}' localhost:8080/tasks/1   -> 200, new representation
  $ curl -si -X DELETE localhost:8080/tasks/1                -> 204 No Content (a second DELETE -> 404)
  $ curl -si -X PATCH localhost:8080/tasks/1                 -> 405 Method Not Allowed (Allow: DELETE, GET, PUT)
  ```

### 3.2 Validation and ProblemDetails
- **Goal:** declarative rules, one error shape, 422 for rule violations
- **Run:** `dotnet run --project modules/03-rest-api-design/examples -- 02-validation`
- **Code:** [02_Validation.cs](examples/02_Validation.cs), [Shared/Problems.cs](examples/Shared/Problems.cs)
  ```csharp
  [Required(AllowEmptyStrings = true, ErrorMessage = "is required"), TrimmedLength(1, 200)]
  public string? Title { get; init; }
  ...
  builder.Services.AddValidation();   // checks the parameters of every endpoint, handler runs only when valid
  o.CustomizeProblemDetails = Problems.Customize;   // 400 -> 422 and snake_case field names
  ```
- **Walkthrough:** `AddValidation` produces `ValidationProblemDetails` with status 400 and property names by default;
  `Problems.Customize` is the single place that rewrites them to the Task Tracker contract. JSON that cannot be
  deserialized (unknown enum string, bad date, truncated body) never reaches validation and stays a plain 400.
- **Expected output:**
  ```
  $ curl -si -X POST -H 'Content-Type: application/json' -d '{"title":"  ","tags":["A","a","b"]}' localhost:8080/tasks | sed -E 's/,"trace_id":"[^"]*"//'
  HTTP/1.1 422 Unprocessable Entity
  Content-Type: application/problem+json

  {"type":"https://www.rfc-editor.org/rfc/rfc9110#section-15.5.21","title":"One or more validation errors occurred.","status":422,"errors":{"title":["must be 1-200 characters"],"tags":["every tag must be lowercase"]}}
  $ ... -d '{"tags":[]}' ...        -> 422 {"...","errors":{"title":["is required"]}}
  $ ... -d '{"title":"x","status":"nope"}' ...   -> 400 {"type":"https://tools.ietf.org/html/rfc9110#section-15.5.1","title":"Bad Request","status":400}
  ```

### 3.3 Pagination, filtering and sorting
- **Goal:** bounded, allow-listed list parameters and the `{items,total,limit,offset}` envelope
- **Run:** `dotnet run --project modules/03-rest-api-design/examples -- 03-pagination` (25 seeded tasks)
- **Code:** [03_Pagination.cs](examples/03_Pagination.cs)
  ```csharp
  var limit = Math.Min(query.Limit ?? DefaultLimit, MaxLimit);              // clamp, never trust the client
  ["due_date"] = q => q.OrderBy(t => t.DueDate is null).ThenBy(t => t.DueDate).ThenBy(t => t.Id),   // total order
  ```
- **Walkthrough:** `[AsParameters]` binds a record from the query string. Sort keys come from a dictionary
  (allow-list) and always end with `Id` so pages are stable. `limit=1000` is clamped, `limit=0` is a 400.
- **Expected output** (`| grep -oE '"(id|total|limit|offset)":[0-9]+'` keeps it short):
  ```
  /tasks?limit=2                         id 25, 24         total 25  limit 2   offset 0
  /tasks?limit=2&offset=2&sort=created_at id 3, 4          total 25  limit 2   offset 2
  /tasks?limit=1000&status=done          8 items, limit 100 (clamped)
  /tasks?offset=100                      {"items":[],"total":25,"limit":20,"offset":100}
  /tasks?limit=0      -> 400  "detail":"limit must be at least 1"
  /tasks?limit=abc    -> 400  (binding failure)
  /tasks?sort=title   -> 400  "detail":"sort must be one of: created_at, -created_at, due_date, -due_date"
  /tasks?status=1     -> 400  "detail":"status must be todo, in_progress or done"
  ```

### 3.4 Versioning and idempotency
- **Goal:** evolve the API without breaking clients; make retries of `POST` safe
- **Run:** `dotnet run --project modules/03-rest-api-design/examples -- 04-versioning-idempotency`
- **Code:** [04_VersioningIdempotency.cs](examples/04_VersioningIdempotency.cs)
- **Walkthrough:** the version is part of the route (`/v1/tasks`, `/v2/tasks`) via two route groups. The v1 group's
  filter adds `Deprecation`, `Sunset` and `Link: rel="successor-version"`. For `POST /v2/tasks` an `Idempotency-Key`
  header makes a retry return the first result; a different body under the same key is a 422. The store keeps a
  `Lazy<Entry>` per key because `ConcurrentDictionary.GetOrAdd(key, factory)` may run the factory twice.
- **Expected output:**
  ```
  $ curl -si -X POST -H 'Content-Type: application/json' -H 'Idempotency-Key: abc-1' -d '{"title":"Pay rent"}' localhost:8080/v2/tasks
  HTTP/1.1 201 Created
  Location: /v2/tasks/1

  {"id":1,"title":"Pay rent","status":"todo","due_date":null,"tags":[],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}
  $ (same request again)
  HTTP/1.1 201 Created
  Location: /v2/tasks/1
  Idempotent-Replayed: true           <- same task, nothing was created
  $ (same key, -d '{"title":"Other"}')
  HTTP/1.1 422 Unprocessable Entity
  {"...","errors":{"idempotency-key":["was already used with a different request body"]}}
  $ curl -si localhost:8080/v1/tasks
  HTTP/1.1 200 OK
  Deprecation: true
  Sunset: Sat, 31 Dec 2033 23:59:59 GMT
  Link: </v2/tasks>; rel="successor-version"

  [{"id":1,"title":"Pay rent","done":false}]
  ```
  Six parallel `curl`s with one key create exactly one task.

### 3.5 The Task Tracker API (PUT, PATCH, list, OpenAPI)
- **Goal:** everything together, in memory (the capstone P2 adds SQLite in module 05)
- **Run:** `dotnet run --project modules/03-rest-api-design/examples -- 05-task-tracker-api`
- **Code:** [05_TaskTrackerApi.cs](examples/05_TaskTrackerApi.cs)
- **Walkthrough:** `PATCH` binds a `JsonElement` because a record cannot tell "member absent" (leave alone) from
  `"due_date": null` (clear it). The merged value is validated with `Validator.TryValidateObject` through the same attributes,
  so PATCH and POST share rules. `MapOpenApi()` serves `/openapi/v1.json`; `WithSummary`/`WithTags` document endpoints.
- **Expected output:**
  ```
  $ curl -X PATCH -d '{"status":"in_progress"}' localhost:8080/tasks/1
  {"id":1,"title":"Write docs","status":"in_progress","due_date":"2030-01-31","tags":["docs","p1"],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"} [200]
  $ curl -X PATCH -d '{"due_date":null,"extra":"ignored"}' localhost:8080/tasks/1
  {"id":1,"title":"Write docs","status":"in_progress","due_date":null,"tags":["docs","p1"],...} [200]
  $ curl -X PATCH -d '{"title":""}' localhost:8080/tasks/1                -> 422 {"...","errors":{"title":["must be 1-200 characters"]}}
  $ curl -X PATCH -d '{"due_date":"tomorrow"}' localhost:8080/tasks/1     -> 400 "a field has the wrong type or format"
  $ curl -X PATCH -d '{}' localhost:8080/tasks/9                          -> 404 "task 9 does not exist"
  $ curl -X POST -d '{"title":"","tags":["A","b","b"]}' localhost:8080/tasks
  {"...","status":422,"errors":{"title":["must be 1-200 characters"],"tags":["every tag must be lowercase"]}}
  $ curl -s localhost:8080/openapi/v1.json | grep -oE '"/[a-z{}/]*"|"summary": "[^"]*"'
  "/tasks"  "summary": "List tasks (filter, sort, paginate)"  "summary": "Create a task"
  "/tasks/{id}"  "summary": "Get one task"  "summary": "Replace a task"  "summary": "Change some fields of a task"  "summary": "Delete a task"
  ```
  The list response is `{"items":[...],"total":1,"limit":20,"offset":0}`.

## 4. Common Pitfalls
| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| Clients set `id`, `created_at`, `is_admin` and it sticks (over-posting) | the handler binds the entity/storage type | bind a dedicated input record; map explicitly |
| `?limit=1000000` loads the whole table | page size taken from the client | clamp to a maximum and give a default |
| Validation returns **400** with `Title` (PascalCase) keys | `AddValidation` defaults to 400 and property names | `CustomizeProblemDetails` to 422 and `SnakeCaseLower` |
| `error CS8785: ... hintName 'ValidatableInfoResolver.g.cs' ... must be unique` | the `AddValidation` source generator supports one call site per project | call it once (see `ExampleHost.AddValidatedProblemDetails`) |
| `Enum.TryParse("1")` or `"InProgress"` accepted as a status | enums parse numbers and C# names | strict `switch` on wire names (`TaskStates.TryParse`) |
| `PATCH` clears fields that were not sent | a record/POCO cannot distinguish absent from `null` | bind `JsonElement`/`JsonNode` and merge only present members |
| Pages overlap or skip rows | sort by a non-unique column (e.g. `due_date`) with offset paging | always add a unique tie-breaker (`Id`) |
| `GetOrAdd(key, factory)` ran the side effect twice | the factory is not atomic | store a `Lazy<T>`/`TaskCompletionSource` and use the winner |
| Some errors are JSON problems, others empty bodies or HTML | bare `Results.NotFound()`/framework 4xx have no body | `AddProblemDetails()` + `UseStatusCodePages()`, return `Problems.*` |
| Returning `200` with `{"error": ...}` | habit from RPC-style APIs | use the status code; clients and proxies rely on it |

## 5. Coming From Other Languages
- **Java/Spring:** `@Valid` + Bean Validation is `AddValidation` + DataAnnotations; `ResponseEntity<T>` is `Results<...>`; `@ControllerAdvice` problem handling is `IProblemDetailsService`/`CustomizeProblemDetails`.
- **Go:** there is no tag-based validation in the stdlib; here attributes play the role of `validate:"..."` tags, but run in the framework before your handler.
- **TypeScript (Express/Nest):** DTO classes + `class-validator` map to input records + DataAnnotations; `{ message }` error shapes map to RFC 9457.
- **Python (FastAPI):** the 422 + `errors` shape is the same idea as FastAPI's `RequestValidationError`; `Optional[...]` vs missing needs the same PATCH care.

## 6. Exercises
Tests use an in-memory `TestServer` (no sockets); Ex1 is pure logic.

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Paging and sorting | Easy | bounded pagination, sort allow-list, list envelope | `dotnet test modules/03-rest-api-design/exercises --filter "FullyQualifiedName~.Ex1_"` |
| 2 | Task validation | Medium | field rules, all errors at once, 422 problem document | `dotnet test modules/03-rest-api-design/exercises --filter "FullyQualifiedName~.Ex2_"` |
| 3 | Idempotency store | Stretch | `Idempotency-Key`, replay, mismatch, concurrency, failure handling | `dotnet test modules/03-rest-api-design/exercises --filter "FullyQualifiedName~.Ex3_"` |

### Exercise 1: Paging and sorting ([Ex1_Paging.cs](exercises/Ex1_Paging.cs))
- **Task:** implement `PageRequest.TryCreate`, `Page<T>.Of` and `SortSpec.TryParse`.
- **Acceptance:** `Ex1_PagingTests` (defaults, clamping, rejection, slicing past the end, one enumeration, JSON shape, allow-list).
- **Hint:** <details><summary>show</summary>Materialize the source once (`ToList`), then `Skip(offset).Take(limit)`; strip a leading `-` before checking the allow-list.</details>

### Exercise 2: Task validation ([Ex2_TaskValidation.cs](exercises/Ex2_TaskValidation.cs))
- **Task:** implement `TaskValidator.Validate` so that every rule listed in its XML doc produces the documented message under the snake_case field name; the endpoint is provided.
- **Acceptance:** `Ex2_TaskValidationTests` (one case per rule, trimming, several errors at once, 422 `application/problem+json`, 201 + `Location`, 400 for malformed JSON).
- **Hint:** <details><summary>show</summary>A small local `Add(field, message)` that appends to a `Dictionary<string, List<string>>` keeps each rule to one line; `DateOnly.TryParseExact(..., "yyyy-MM-dd", CultureInfo.InvariantCulture, ...)` rejects `2030-1-3`.</details>

### Exercise 3: Idempotency store ([Ex3_Idempotency.cs](exercises/Ex3_Idempotency.cs))
- **Task:** implement `IdempotencyStore.ExecuteAsync` (run once per key, replay, mismatch, concurrent callers wait, failures are forgotten); the endpoint is provided.
- **Acceptance:** `Ex3_IdempotencyTests`, including three concurrent calls running the action once.
- **Hint:** <details><summary>show</summary>`ConcurrentDictionary.GetOrAdd(key, new Entry(fingerprint, tcs.Task))` and compare the stored task with your own to learn whether you won; on exception remove the key before failing the task.</details>

## 7. Recap
- Resources are nouns; the verb + status code carry the semantics; errors are `application/problem+json` with the same status codes everywhere.
- Separate input type, storage type and output type; never bind the entity.
- Every list endpoint needs a default, a maximum, an allow-listed sort and a stable order.
- 400 = could not be parsed, 422 = parsed but invalid, 409 = conflicts with state.
- Retries are a fact of life: give non-idempotent `POST`s an idempotency key; version routes before you need to.

## 8. Self-check Questions
1. A client sends `{"status":"nope"}` and gets a 400, but `{"title":""}` gets a 422. Why the difference?
2. `PATCH` with `{"due_date":null}` and with `{}` must behave differently. How does your binding make that possible?
3. Why does the `due_date` sort end with `ThenBy(t => t.Id)`?
4. Is `DELETE` idempotent if the second call returns 404? What does idempotent actually promise?
5. Two retries with the same idempotency key arrive at the same time. Which part of the store prevents two creations?

## 9. Further Reading
- [Minimal API validation (ASP.NET Core 10)](https://learn.microsoft.com/aspnet/core/fundamentals/minimal-apis/parameter-binding#validation-support-in-minimal-apis)
- [Handle errors in minimal APIs / ProblemDetails](https://learn.microsoft.com/aspnet/core/fundamentals/minimal-apis/handle-errors)
- [OpenAPI in ASP.NET Core](https://learn.microsoft.com/aspnet/core/fundamentals/openapi/overview)
- RFC 9457 (Problem Details), RFC 9110 (HTTP semantics), the IETF `Idempotency-Key` header draft

---
**Next:** [04 Serialization](../04-serialization/README.md)
