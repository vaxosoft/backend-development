# 03. REST API Design in TypeScript

> Estimated time: 2 h | Difficulty: Intermediate | Prerequisites: [01 Language Fundamentals](../01-language-fundamentals/README.md), [02 HTTP Server](../02-http-server/README.md)

## Learning Outcomes

After this module you can:

- [ ] Model resources and choose verbs and status codes (`201` + `Location`, `204`, `404`, `409`, `422`)
- [ ] Validate request bodies and query strings at runtime with zod and report field errors
- [ ] Return every failure in one error envelope through a single Fastify error handler
- [ ] Implement pagination, filtering and sorting with sane limits
- [ ] Explain versioning and idempotency, and implement `Idempotency-Key` for `POST`

## 1. Concept Overview

REST maps a domain to _resources_ (nouns: `/tasks/42`) manipulated through a small set of HTTP verbs. `GET` is safe, `PUT` (replace) and `DELETE` are idempotent, `POST` creates and is not, `PATCH` changes part of a resource. Status codes carry the outcome so clients never parse prose: `200` OK, `201` created (+ `Location`), `204` no content, `400` malformed, `404` unknown, `409` conflict, `422` well-formed but invalid, `500` our fault.

A good API is predictable: one error shape, one list shape (`{items,total,limit,offset}`), documented limits (default 20, max 100), stable field names (`snake_case`), and a way to evolve (`/v1/...`). The wire format used in this course (field names, error envelope, list shape) is fixed in `framework/domain-and-projects.md`.

## 2. How TypeScript Approaches It

- **Fastify** routing and hooks (module 02) provide the transport; `setErrorHandler` and `setNotFoundHandler` produce the envelope in one place.
- **zod 4** is the validation library: one runtime schema gives the validator _and_ the static type (`z.infer`). Fastify's built-in JSON-schema validation exists, but zod keeps types and rules together and works outside HTTP too. Generating OpenAPI documents from schemas is possible with extra libraries; it is only mentioned here.
- **Errors** are `Error` subclasses (`HttpError`) thrown anywhere in a handler; the error handler maps them to `{ "error": { "code", "message", "fields"? } }`. Unknown errors become a generic `500`: details go to the log, never to the client.
- **Why not trust types:** `Body: TaskInput` on a route is erased at runtime. Always `safeParse` untrusted data.

Shared code for the examples is in [examples/_shared.ts](examples/_shared.ts): error handling, schemas, `parseListQuery` and `listTasks`.

## 3. Guided Examples

### 3.1 One error envelope

- **Goal:** every error, including framework errors, has the same JSON shape.
- **Run:** `node modules/03-rest-api-design/examples/01_error_envelope.ts` (listens on `ADDR`, default `127.0.0.1:8080`)
- **Code:** [examples/01_error_envelope.ts](examples/01_error_envelope.ts), handler in [examples/_shared.ts](examples/_shared.ts)
- **Walkthrough:** `HttpError` carries its status and `code`. The error handler maps it, turns Fastify's own 4xx errors (invalid JSON, body too large) into `bad_request` / `payload_too_large`, and hides everything else behind `500 internal`. The handler receives `error: unknown`, so it narrows before reading `statusCode`.

```ts
app.setErrorHandler(async (error, req, reply) => {
  if (error instanceof HttpError) {
    return reply.code(error.statusCode).send(body(error.code, error.message, error.fields));
  }
  ...
  req.log.error(error); // details go to the log, never to the client
  return reply.code(500).send(body('internal', 'internal server error'));
});
```

- **Expected output** (the `Date` etc. headers are omitted):

  ```text
  $ curl -i http://127.0.0.1:8080/tasks/7
  HTTP/1.1 404 Not Found
  content-type: application/json; charset=utf-8

  {"error":{"code":"not_found","message":"task 7 not found"}}
  $ curl -i http://127.0.0.1:8080/conflict
  HTTP/1.1 409 Conflict
  content-type: application/json; charset=utf-8

  {"error":{"code":"conflict","message":"slug already taken"}}
  $ curl -i http://127.0.0.1:8080/boom
  HTTP/1.1 500 Internal Server Error
  content-type: application/json; charset=utf-8

  {"error":{"code":"internal","message":"internal server error"}}
  $ curl -i http://127.0.0.1:8080/nope
  HTTP/1.1 404 Not Found
  content-type: application/json; charset=utf-8

  {"error":{"code":"not_found","message":"route GET /nope not found"}}
  $ curl -i -X POST -H 'content-type: application/json' -d '{oops' http://127.0.0.1:8080/echo
  HTTP/1.1 400 Bad Request
  content-type: application/json; charset=utf-8

  {"error":{"code":"bad_request","message":"malformed request"}}
  ```

  A JSON body longer than the 256-byte `bodyLimit` in this example gives `413` with `payload_too_large`, and a body sent without a JSON content type gives `415` with `bad_request`.

### 3.2 Validation with zod

- **Goal:** turn untrusted `unknown` into a typed value, or into a `fields` map.
- **Run:** `node modules/03-rest-api-design/examples/02_validation_zod.ts`
- **Code:** [examples/02_validation_zod.ts](examples/02_validation_zod.ts), schemas in [examples/_shared.ts](examples/_shared.ts)
- **Walkthrough:** the schema trims the title, applies defaults, checks calendar dates (`2030-02-30` is rejected), tag rules and `owner_id`. `safeParse` returns issues with a `path`; `zodFields` joins the path (`tags.0`) and keeps the first message. Unknown fields are stripped. For `PATCH`, fields are `optional()` _without_ defaults, so "absent" (unchanged) and `null` (clear) stay distinguishable. The last lines show why a type annotation proves nothing about parsed JSON.

```ts
export const taskInputSchema = z.object(
  {
    title,
    status: status.default('todo'),
    due_date: dueDate.default(null),
    tags: tags.default([]),
    owner_id: ownerId.default(null),
  },
  'must be a JSON object',
);
export type TaskInput = z.infer<typeof taskInputSchema>;
```

- **Expected output:**

  ```text
  {
    title: 'Write docs',
    status: 'todo',
    due_date: null,
    tags: [ 'docs', 'p1' ],
    owner_id: null
  }
  {
    title: 'must be 1-200 characters',
    status: 'must be one of todo, in_progress, done',
    due_date: 'must be a date like 2030-01-31',
    'tags.0': 'must be lowercase',
    'tags.3': 'must be a string',
    owner_id: 'must be an integer'
  }
  null { body: 'must be a JSON object' }
  "text" { body: 'must be a JSON object' }
  [] { body: 'must be a JSON object' }
  {
    title: 'x',
    status: 'todo',
    due_date: null,
    tags: [],
    owner_id: null
  }
  { title: 'New title' }
  { due_date: null }
  false
  number <- the compiler believes it is a string
  ```

### 3.3 Pagination, filtering, sorting

- **Goal:** list endpoints with predictable limits.
- **Run:** `node modules/03-rest-api-design/examples/03_pagination_filter_sort.ts`
- **Code:** [examples/03_pagination_filter_sort.ts](examples/03_pagination_filter_sort.ts)
- **Walkthrough:** query-string values are strings, so `limit` is matched with `/^\d+$/` before converting (`Number('')` is `0`, `Number(undefined)` is `NaN`). Values above 100 are clamped, `0`/negative/non-numeric values and repeated parameters are `400`. `sort` is a whitelist (`created_at`, `due_date`, optional `-` prefix); tasks without a due date always sort last; ties break by `id` so pages are stable. `total` counts matches _before_ slicing.

```ts
limit: wholeNumber
  .refine((n) => n >= 1, 'must be at least 1')
  .transform((n) => Math.min(n, 100)) // larger values are clamped, not rejected
  .default(20),
```

- **Expected output:**

  ```text
  defaults                     ids=[5,4,3,2,1] total=5 limit=20 offset=0
  status=todo                  ids=[5,3,1] total=3 limit=20 offset=0
  tag=docs                     ids=[3,1] total=2 limit=20 offset=0
  sort=due_date                ids=[4,3,1,2,5] total=5 limit=20 offset=0
  sort=-due_date               ids=[1,3,4,2,5] total=5 limit=20 offset=0
  limit=2&offset=2             ids=[3,2] total=5 limit=2 offset=2
  limit=1000 (clamped)         ids=[5,4,3,2,1] total=5 limit=100 offset=0
  offset past the end          ids=[] total=5 limit=20 offset=50
  limit=abc                    400 invalid query parameter limit: must be a non-negative integer
  limit=0                      400 invalid query parameter limit: must be at least 1
  limit=-1                     400 invalid query parameter limit: must be a non-negative integer
  limit=1&limit=2 (repeated)   400 invalid query parameter limit: must be given once
  sort=title (not allowed)     400 invalid query parameter sort: unsupported sort
  NaN 0 NaN <- what Number() makes of missing values
  ```

### 3.4 The Tasks API

- **Goal:** all pieces together in the Task Tracker domain: CRUD, `201` + `Location`, `PUT` vs `PATCH`, `422` field errors, filtering, versioning and idempotency.
- **Run:** `node modules/03-rest-api-design/examples/04_tasks_api.ts`
- **Code:** [examples/04_tasks_api.ts](examples/04_tasks_api.ts)
- **Walkthrough:** the routes are a plugin registered with `{ prefix: '/v1' }`, so a future `/v2` can live beside it. Handlers call `parseOr422(schema, body, 'task')` and `parseListQuery(req.query)` and simply throw on failure. `PUT` rebuilds the task from defaults; `PATCH` copies only fields that are not `undefined`. For `POST` with an `Idempotency-Key`, the first response is remembered (a fingerprint of the validated body plus the task); a retry returns the same task with `idempotent-replayed: true`, and a different body under the same key is a `409`. The in-memory clock counts seconds so output is deterministic.
- **Expected output** (task bodies shortened where noted):

  ```text
  $ curl -i -X POST -H 'content-type: application/json' -d '{"title":"  Write docs  ","tags":["docs","p1"],"due_date":"2030-01-31"}' http://127.0.0.1:8080/v1/tasks
  HTTP/1.1 201 Created
  location: /v1/tasks/1
  content-type: application/json; charset=utf-8

  {"id":1,"title":"Write docs","status":"todo","due_date":"2030-01-31","tags":["docs","p1"],"owner_id":null,"created_at":"2030-01-01T09:00:01Z"}
  $ curl -i -X POST -H 'content-type: application/json' -H 'idempotency-key: abc' -d '{"title":"Ship it"}' http://127.0.0.1:8080/v1/tasks
  HTTP/1.1 201 Created
  location: /v1/tasks/2
  ...
  {"id":2,"title":"Ship it","status":"todo","due_date":null,"tags":[],"owner_id":null,"created_at":"2030-01-01T09:00:02Z"}
  $ (same command again)
  HTTP/1.1 201 Created
  location: /v1/tasks/2
  idempotent-replayed: true
  ...
  {"id":2,"title":"Ship it", ...}
  $ (same key, body {"title":"Other"})
  HTTP/1.1 409 Conflict
  {"error":{"code":"conflict","message":"Idempotency-Key was already used with a different body"}}
  $ curl -i -X POST -H 'content-type: application/json' -d '{"title":"","tags":["A"],"status":"x"}' http://127.0.0.1:8080/v1/tasks
  HTTP/1.1 422 Unprocessable Entity
  {"error":{"code":"validation_failed","message":"invalid task","fields":{"title":"must be 1-200 characters","status":"must be one of todo, in_progress, done","tags.0":"must be lowercase"}}}
  $ curl -X PATCH -H 'content-type: application/json' -d '{"status":"done","due_date":null}' http://127.0.0.1:8080/v1/tasks/1
  {"id":1,"title":"Write docs","status":"done","due_date":null,"tags":["docs","p1"],"owner_id":null,"created_at":"2030-01-01T09:00:01Z"}
  $ curl -X PUT -H 'content-type: application/json' -d '{"title":"Replaced"}' http://127.0.0.1:8080/v1/tasks/1
  {"id":1,"title":"Replaced","status":"todo","due_date":null,"tags":[],"owner_id":null,"created_at":"2030-01-01T09:00:01Z"}
  $ curl "http://127.0.0.1:8080/v1/tasks?sort=created_at&limit=1"
  {"items":[{"id":1,"title":"Replaced", ...}],"total":2,"limit":1,"offset":0}
  $ curl -i "http://127.0.0.1:8080/v1/tasks?limit=abc"
  HTTP/1.1 400 Bad Request
  {"error":{"code":"bad_request","message":"invalid query parameter limit: must be a non-negative integer"}}
  $ curl -i -X DELETE http://127.0.0.1:8080/v1/tasks/2
  HTTP/1.1 204 No Content
  $ curl -i http://127.0.0.1:8080/v1/tasks/2
  HTTP/1.1 404 Not Found
  {"error":{"code":"not_found","message":"task 2 not found"}}
  $ curl -i http://127.0.0.1:8080/tasks
  HTTP/1.1 404 Not Found
  {"error":{"code":"not_found","message":"route GET /tasks not found"}}
  ```

## 4. Common Pitfalls

| Mistake                                                      | Why it happens                                                          | Fix                                                                |
| ------------------------------------------------------------ | ----------------------------------------------------------------------- | ------------------------------------------------------------------ |
| Trusting `Body: TaskInput` on a route                        | generics are erased; Fastify does not validate unless a schema is given | `schema.safeParse(req.body)` and answer `422`                      |
| `limit=abc` becomes `NaN` and `slice(0, NaN)` returns `[]`   | `Number(undefined)`/`Number('abc')` is `NaN`, `Number('')` is `0`       | match `/^\d+$/` first, reject otherwise                            |
| `200` with `{"error": ...}`                                  | errors written by hand in each handler                                  | throw `HttpError`; one `setErrorHandler` sets the status           |
| Leaking `error.message` (SQL, paths, secrets) in `500`s      | forwarding the caught error                                             | log it, answer a fixed `internal` message                          |
| `PATCH` cannot tell "absent" from `null`                     | defaults or `?? ` operators erase the difference                        | use `optional()` schemas without defaults and test `!== undefined` |
| Validation defaults overwrite `PATCH` fields                 | `.partial()` on a schema that has `.default()` re-applies defaults      | build the PATCH schema from default-free field schemas             |
| Repeated query parameters (`?limit=1&limit=2`) become arrays | Fastify parses duplicates into `string[]`                               | schemas expect `string`, so arrays are rejected as `400`           |
| Unstable pages                                               | sorting by a non-unique key                                             | add the id as tie-breaker                                          |
| `Array.prototype.sort` mutating the stored list              | in-place sort on the source array                                       | filter/slice first (a new array) or use `toSorted`                 |

## 5. Coming From Other Languages

- **Python FastAPI / pydantic:** zod is the pydantic equivalent, but you call it yourself; FastAPI's automatic 422 body is replaced by the shared envelope here too.
- **Go:** `json.Decoder` + manual checks (or validator tags) ~ `safeParse`; `errors.As` in a middleware ~ `instanceof` in `setErrorHandler`.
- **Java/C#:** Bean Validation / DataAnnotations ~ zod refinements. C# teaches `ProblemDetails` (RFC 9457) instead of this envelope; the status codes are identical.
- **Express:** a 4-argument error middleware ~ `setErrorHandler`.

## 6. Exercises

| #   | Title                 | Difficulty | Skill practiced                                         | Run tests                                                                        |
| --- | --------------------- | ---------- | ------------------------------------------------------- | -------------------------------------------------------------------------------- |
| 1   | Pagination parameters | Easy       | query parsing, clamping, `NaN` traps                    | `node --test "modules/03-rest-api-design/exercises/ex1_pagination/**/*.test.ts"` |
| 2   | Validate a task       | Medium     | zod schemas, defaults, `fields` map                     | `node --test "modules/03-rest-api-design/exercises/ex2_validation/**/*.test.ts"` |
| 3   | Tasks API             | Stretch    | envelope, status codes, PUT/PATCH, filters, idempotency | `node --test "modules/03-rest-api-design/exercises/ex3_tasks_api/**/*.test.ts"`  |

### Exercise 1: Pagination parameters

- **Task:** implement `parsePageParams` and `paginate` in [pagination.ts](exercises/ex1_pagination/pagination.ts).
- **Acceptance:** [pagination.test.ts](exercises/ex1_pagination/pagination.test.ts) passes: defaults, clamp at 100, rejection of `""`, `" 5"`, `1.5`, `-1`, `1e3`, `0` limit and repeated parameters, offsets past the end.
- **Hint:** <details><summary>Show</summary>test the string with `/^\d+$/` before calling `Number`.</details>

### Exercise 2: Validate a task

- **Task:** implement `validateTaskInput` in [validation.ts](exercises/ex2_validation/validation.ts) with zod; return all problems as `fields`.
- **Acceptance:** [validation.test.ts](exercises/ex2_validation/validation.test.ts) passes: defaults, trimming, real calendar dates, tag rules (`tags` vs `tags.1` keys), non-object bodies under `body`.
- **Hint:** <details><summary>Show</summary>`z.iso.date()`, `z.int()`, `.nullable().default(null)`, and `issue.path.join('.')`.</details>

### Exercise 3: Tasks API

- **Task:** implement `buildApi` in [api.ts](exercises/ex3_tasks_api/api.ts) using the provided [task-model.ts](exercises/ex3_tasks_api/task-model.ts) (schemas and an in-memory store): CRUD routes, the error envelope, a 1 KiB body limit, list filters/sort/pagination and `Idempotency-Key` on `POST`.
- **Acceptance:** [api.test.ts](exercises/ex3_tasks_api/api.test.ts) passes (requests via `inject()`).
- **Hint:** <details><summary>Show</summary>write the error handler first; then every route can simply `throw`. Keep a `Map` from idempotency key to `{ fingerprint, id }`.</details>

## 7. Recap

- Resources are nouns, verbs carry intent, status codes carry the outcome: `201`+`Location`, `204`, `404`, `422` (valid JSON, invalid data) vs `400` (malformed).
- One envelope, produced by one error handler; never leak internals in `500`.
- Validate at the boundary with zod; the schema is also your type.
- Lists need limits, a whitelist for sort fields, and a stable order.
- Version in the path (`/v1`), make unsafe retries safe with `Idempotency-Key`.

## 8. Self-check Questions

1. When is `422` right and when is `400`?
2. Why does `PATCH` need a different schema from `POST`, and what goes wrong if you reuse the default-filled one?
3. What can a client assume after `PUT` twice with the same body? After `POST` twice?
4. Why is `limit=1000` clamped but `limit=abc` rejected?
5. Why does the error handler take `error: unknown`, and how do you read `statusCode` safely?

## 9. Further Reading

- [zod documentation](https://zod.dev/): schemas, refinements, error customization
- [Fastify: Validation and Serialization, Errors](https://fastify.dev/docs/latest/Reference/Errors/)
- [RFC 9110: HTTP Semantics](https://www.rfc-editor.org/rfc/rfc9110) (methods, status codes) and [IETF Idempotency-Key header draft](https://datatracker.ietf.org/doc/draft-ietf-httpapi-idempotency-key-header/)
- [OpenAPI Specification](https://spec.openapis.org/oas/latest.html): documenting an API (not implemented in this course)

---

**Next:** [04 Serialization](../04-serialization/README.md)
