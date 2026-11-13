# P2 - Tasks CRUD API

**Modules exercised:** [02 HTTP server](../../modules/02-http-server/README.md), [03 REST API design](../../modules/03-rest-api-design/README.md), [04 Serialization](../../modules/04-serialization/README.md), [05 Database access](../../modules/05-database-access/README.md)

Build the Task Tracker JSON API on Fastify with SQLite (`better-sqlite3`): create, read, replace, patch, delete and list tasks, with strict validation (`zod`) and the shared error envelope.

## Requirements

Task fields (JSON uses `snake_case`):

| Field        | Rules                                                                                   |
| ------------ | --------------------------------------------------------------------------------------- |
| `id`         | integer, assigned by the server, read-only                                              |
| `title`      | required string, trimmed, 1-200 characters                                              |
| `status`     | `todo` (default), `in_progress`, `done`                                                 |
| `due_date`   | `YYYY-MM-DD` (a real calendar date) or `null` (default)                                 |
| `tags`       | 0-10 strings (default `[]`), each 1-30 characters, lowercase, no duplicates             |
| `owner_id`   | integer or `null` (default); only settable with POST/PUT                                |
| `created_at` | RFC 3339 UTC without fractions (`2030-01-01T09:00:00Z`), read-only, from `config.clock` |

Endpoints:

| Request              | Result                                                                                                                                           |
| -------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| `POST /tasks`        | `201`, `Location: /tasks/{id}`, the task. Unknown/read-only fields are ignored                                                                   |
| `GET /tasks/{id}`    | `200` task; unknown or non-numeric id: `404 not_found`                                                                                           |
| `PUT /tasks/{id}`    | full replace with the same rules as POST (omitted optional fields return to their defaults); keeps `id` and `created_at`                         |
| `PATCH /tasks/{id}`  | only `title`, `status`, `due_date`, `tags` are patchable; an absent key is unchanged; `"due_date": null` clears the date; other keys are ignored |
| `DELETE /tasks/{id}` | `204`, no body                                                                                                                                   |
| `GET /tasks`         | `{"items":[...],"total":N,"limit":L,"offset":O}`                                                                                                 |

List query parameters: `status`, `tag` (exact match), `limit` (default 20, values above 100 are clamped to 100, below 1 is an error), `offset` (default 0), `sort` = `created_at` | `-created_at` (default) | `due_date` | `-due_date`. `total` counts all matches, ignoring `limit`/`offset`. Tasks without a due date come last in both `due_date` sorts. Ties are broken by `id` (descending for `-created_at`, ascending otherwise).

Errors always use the envelope `{"error":{"code","message","fields"?}}`:

| Situation                                                              | Status | `code`                  |
| ---------------------------------------------------------------------- | ------ | ----------------------- |
| malformed JSON, bad query value (`limit=abc`, unknown `sort`/`status`) | 400    | `bad_request`           |
| well-formed but invalid body (`fields` maps field name to message)     | 422    | `validation_failed`     |
| unknown task / route                                                   | 404    | `not_found`             |
| anything unexpected                                                    | 500    | `internal` (no details) |

Persistence: SQLite through `better-sqlite3`; the schema is created at startup ([starter/src/db.ts](./starter/src/db.ts)). Hide storage behind the `TaskRepository` interface. Always use bound parameters, never string-built values.

## Acceptance criteria (test names in [tests/tasks-api.test.ts](./tests/tasks-api.test.ts))

| Area        | Tests                                                                                                                                                                                                                                                                              |
| ----------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Create      | `creates a task: 201, Location header and defaults`, `stores every writable field and trims the title`, `assigns increasing ids and ignores client-supplied id and created_at`                                                                                                     |
| Validation  | `rejects a missing, blank or too long title ...`, `rejects an unknown status, a bad due_date and a non-integer owner_id`, `rejects bad tags ...`, `malformed JSON is a 400 bad_request, a well-formed non-object is a 422`                                                         |
| SQL safety  | `quotes in titles and tags are stored as data, not as SQL`                                                                                                                                                                                                                         |
| Read        | `returns the task; unknown and non-numeric ids are 404 not_found`                                                                                                                                                                                                                  |
| Replace     | `replaces the whole task: omitted optional fields go back to their defaults`, `validates the body (422) and reports unknown ids (404)`                                                                                                                                             |
| Patch       | `changes only the supplied fields`, `due_date: null clears the date ...`, `replaces tags and ignores fields that are not patchable`, `validates supplied fields (422) ...`                                                                                                         |
| Delete      | `answers 204 with no body, removes the task, and a second delete is 404`                                                                                                                                                                                                           |
| List        | `returns the list envelope, newest first by default ...`, `filters by status and by tag ...`, `paginates with limit and offset ...`, `a limit above 100 is clamped to 100`, `bad query values are 400 bad_request`, `sorts by created_at ascending, and by due_date both ways ...` |
| Persistence | `tasks survive closing and reopening the database file`                                                                                                                                                                                                                            |

## How to run

From `languages/typescript/`:

```bash
node projects/P2-tasks-crud-api/starter/src/main.ts
```

Environment variables: `ADDR` (`host:port`, default `127.0.0.1:8080`) and `DB_PATH` (SQLite file, default `tasks.db` in the current directory; delete it to start fresh).

```bash
curl -i -X POST http://127.0.0.1:8080/tasks -H 'content-type: application/json' \
  -d '{"title":"Write docs","tags":["docs","p2"],"due_date":"2030-01-31"}'
curl -X PATCH http://127.0.0.1:8080/tasks/1 -H 'content-type: application/json' -d '{"status":"done","due_date":null}'
curl 'http://127.0.0.1:8080/tasks?tag=docs&limit=5'
curl -X POST http://127.0.0.1:8080/tasks -H 'content-type: application/json' -d '{"title":""}'
```

With the reference solution the last request answers `{"error":{"code":"validation_failed","message":"invalid task","fields":{"title":"must be 1-200 characters"}}}`, and the first has `HTTP/1.1 201 Created` and `location: /tasks/1`. The server logs one pino JSON line per request.

## How to run the tests

```bash
node --test "projects/P2-tasks-crud-api/tests/**/*.test.ts"
```

Out of the box every test fails (`Error: TODO` surfaces as a `500`, missing routes as `404`). The tests call `buildApp({ dbPath: ':memory:', clock })` and use `app.inject()`, so no port is needed.

## Suggested steps

1. [starter/src/validation.ts](./starter/src/validation.ts): write the three parsers with `zod`; turn issues into the `fields` map (first message per top-level field).
2. [starter/src/repository.ts](./starter/src/repository.ts): implement `create`, `get`, `remove`, `replace`, `patch`, `list` with prepared statements.
3. [starter/src/routes.ts](./starter/src/routes.ts): fill in the TODO routes (`GET /tasks/:id` shows the style).
4. Run the tests after each step; the order above lets the create/read tests pass first.

<details>
<summary>Hints</summary>

- `z.object({...}, { error: 'must be a JSON object' })` customises the message for a non-object body; `.default(...)` fills omitted fields; `.optional()` keeps a key absent.
- `.nullable()` allows `null`, `.optional()` allows absence: PATCH needs both for `due_date`, but only `.optional()` for `title`.
- A real date check: ``new Date(`${s}T00:00:00Z`).toISOString().startsWith(s)`` rejects `2030-02-31`.
- Query values arrive as strings (or arrays when repeated): convert and validate them, do not trust `Number(x)` (`Number('')` is `0`, `Number('abc')` is `NaN`).
- Store `tags` as a JSON text column; filter with `EXISTS (SELECT 1 FROM json_each(tasks.tags) WHERE value = ?)`.
- `ORDER BY` columns cannot be bound parameters: map the validated `sort` value to a fixed SQL fragment.
- `db.prepare(sql).run(...)` returns `{ changes, lastInsertRowid }`; `changes === 0` means "no such row".
- `db.transaction(fn)` returns a function; call it to run `fn` atomically (use it for read-modify-write in PATCH).
- Throw `HttpError` helpers from [starter/src/errors.ts](./starter/src/errors.ts); the error handler turns them into the envelope.

</details>

## Stretch goals (not tested)

- A `users` table and a foreign key for `owner_id` (`404`/`422` for unknown owners).
- `ETag` / `If-Match` on `PUT` and `PATCH` to detect lost updates.
- `GET /openapi.json` generated from the zod schemas.
- A second `TaskRepository` implementation (in memory) that passes the same contract tests.
- Run the schema from a `PRAGMA user_version` based migration list instead of `CREATE TABLE IF NOT EXISTS`.

## Reference solution

The model answer lives in [solution/](./solution/) (`validation.ts`, `repository.ts`, `routes.ts`): copy its files over `starter/` (same relative paths) and the tests pass. Try the project yourself first.
