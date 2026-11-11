# P2. Tasks CRUD API

> Modules exercised: [02 HTTP server](../../modules/02-http-server/README.md), [03 REST API design](../../modules/03-rest-api-design/README.md), [04 Serialization](../../modules/04-serialization/README.md), [05 Database access](../../modules/05-database-access/README.md)

## Goal

A JSON CRUD API for the Task Tracker domain on ASP.NET Core Minimal APIs, persisted in SQLite through a repository abstraction. You practise validation (422 vs 400), the PUT/PATCH distinction (absent vs `null`), list filtering/sorting/pagination, and safe SQL.

## Requirements

Task wire format (snake_case): `id`, `title`, `status`, `due_date`, `tags`, `owner_id`, `created_at`. Example:

```json
{"id":1,"title":"Write docs","status":"todo","due_date":"2030-01-31","tags":["docs","p2"],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}
```

| Endpoint | Behaviour |
|----------|-----------|
| `POST /tasks` | validate, store, `201` + `Location: /tasks/{id}` + task |
| `GET /tasks/{id}` | `200` task, unknown id `404` |
| `PUT /tasks/{id}` | full replace: absent fields reset to defaults (`status` `todo`, `due_date` `null`, `tags` `[]`, `owner_id` `null`); `id` and `created_at` never change. `200` or `404` |
| `PATCH /tasks/{id}` | only `title`, `status`, `due_date`, `tags` may change; **absent = unchanged**, `"due_date": null` clears it; other fields are ignored. The merged result is validated. `200` or `404` |
| `DELETE /tasks/{id}` | `204` or `404` |
| `GET /tasks` | `?status=&tag=&limit=&offset=&sort=` -> `{"items":[...],"total":N,"limit":L,"offset":O}` |
| `GET /openapi.json` | built-in OpenAPI document (C# only, optional in the spec) |

Validation (`422` `application/problem+json` with `errors: { "<json field>": ["message"] }`):

- `title`: required, 1-200 characters after trimming (the trimmed value is stored)
- `status`: `todo` (default), `in_progress`, `done`
- `due_date`: `null`/absent or a real date formatted `YYYY-MM-DD`
- `tags`: at most 10; each 1-30 characters, lowercase (reject, do not normalise); no duplicates

Other errors, all `application/problem+json`: malformed JSON or wrong JSON types `400`; unknown id `404`; bad query values `400` (`limit` not an integer or `< 1`, `offset` not an integer or `< 0`, unknown `status`, unknown `sort`).

List rules: `limit` default 20, values above 100 are clamped to 100; `offset` default 0; `total` counts all rows matching the filters. `sort` is `created_at`, `-created_at` (default), `due_date` or `-due_date`; tasks without a due date come last in both directions, ties are broken by `id`. `tag` matches tasks that contain that exact tag.

> Errors use `ProblemDetails` / `ValidationProblemDetails` (RFC 9457), the idiomatic ASP.NET Core form of the track's error envelope, with the same status codes.

## Acceptance criteria -> tests

| Criterion | Tests (`tests/TasksApiTests.cs`) |
|-----------|-----------------------------------|
| create | `Create_Returns201WithLocationAndTheStoredTask`, `Create_AppliesDefaults`, `Create_StoresAllFieldsAndTrimsTheTitle` |
| validation | `Create_WithInvalidData_Returns422NamingTheField`, `Create_EnforcesTheTitleLengthLimit`, `Patch_WithInvalidValue_Returns422`, `Put_WithInvalidData_Returns422AndChangesNothing` |
| malformed input | `Create_WithMalformedJson_Returns400` |
| read / unknown ids | `Get_ReturnsTheTask`, `UnknownId_Returns404ProblemJson` |
| replace | `Put_ReplacesEveryFieldAndKeepsCreatedAt` |
| patch | `Patch_ChangesOnlyTheProvidedFields`, `Patch_WithNullDueDate_ClearsIt`, `Patch_IgnoresFieldsItDoesNotOwn` |
| delete | `Delete_Returns204ThenTheTaskIsGone` |
| list envelope and paging | `List_ReturnsTheEnvelopeWithDefaults`, `List_PaginatesAndReportsTheUnpagedTotal`, `List_ClampsLimitAbove100` |
| list sort | `List_DefaultsToNewestFirst`, `List_SortCreatedAt_OldestFirst`, `List_SortDueDate_PutsTasksWithoutDueDateLast` |
| list filters | `List_FiltersByStatus`, `List_FiltersByTag` |
| bad query | `List_WithBadQueryValues_Returns400` |
| OpenAPI | `OpenApiDocument_IsServed` |

## Run

From `languages/csharp/`:

```bash
export DOTNET_NOLOGO=1 DOTNET_CLI_TELEMETRY_OPTOUT=1
ADDR=127.0.0.1:8080 ConnectionStrings__Tasks="Data Source=tasks.db" \
  dotnet run --project projects/P2-tasks-crud-api/starter
```

`ADDR` defaults to `127.0.0.1:8080`; `ConnectionStrings:Tasks` (env var `ConnectionStrings__Tasks`) defaults to the file `tasks.db` in the current directory (the schema is created at startup). Try the reference solution:

```bash
curl -si -X POST localhost:8080/tasks -H 'Content-Type: application/json' \
  -d '{"title":"Write docs","tags":["docs","p2"],"due_date":"2030-01-31"}'
# HTTP/1.1 201 Created
# Location: /tasks/1
# {"id":1,"title":"Write docs","status":"todo","due_date":"2030-01-31","tags":["docs","p2"],"owner_id":null,"created_at":"<now>"}

curl -si -X POST localhost:8080/tasks -H 'Content-Type: application/json' -d '{"title":"","tags":["Docs"]}'
# HTTP/1.1 422 Unprocessable Entity
# {"type":"...","title":"Invalid task","status":422,"errors":{"title":["must be 1-200 characters"],"tags":["tags must be lowercase"]},"trace_id":"..."}

curl -s -X PATCH localhost:8080/tasks/1 -H 'Content-Type: application/json' -d '{"status":"done","due_date":null}'
curl -s 'localhost:8080/tasks?status=done&sort=-created_at&limit=5'
```

## Test

```bash
dotnet test projects/P2-tasks-crud-api/tests
```

Each test builds its own app with `WebApplicationFactory<Program>`, a private shared-cache in-memory SQLite database and a `FakeTimeProvider`, so tests are isolated and deterministic. The starter compiles; the `TODO` stubs in `TaskValidator.cs`, `SqliteTaskRepository.cs` and `TaskEndpoints.cs` throw until you implement them (the tests then fail with 500s and wrong status codes). Models, wiring (`Program.cs`), JSON options, the repository interface and the schema are given.

## Steps and hints

1. `TaskValidator.Validate`, then `Apply` (PATCH).
2. `SqliteTaskRepository`: create/get/replace/delete, then list.
3. `TaskEndpoints`: create/get/delete, replace/patch, list.

<details>
<summary>Hints</summary>

- Return `Results.ValidationProblem(errors, statusCode: 422)` for validation and `Results.Problem(statusCode: 404, ...)` for unknown ids.
- `TaskPatch` already tells you which properties were present (`HasDueDate`, ...). It uses the C# 14 `field` keyword in its setters: STJ calls a setter only for properties that appear in the JSON, including explicit `null`.
- Easiest PATCH: build a `TaskRequest` from the existing task overridden by the present patch fields, then run `Validate` on it.
- SQLite: `INSERT ... RETURNING id, ...` and `UPDATE ... RETURNING ...` give you the row without a second query. Always use `@parameters`; pass `DBNull.Value` for `null`.
- `ORDER BY` cannot be a parameter: map the validated `sort` string to a fixed SQL fragment with a `switch`. `due_date IS NULL, due_date` puts NULLs last.
- Tags are a JSON array in one column; filter with `EXISTS (SELECT 1 FROM json_each(tasks.tags) WHERE value = @tag)`.
- `total` needs a separate `COUNT(*)` with the same `WHERE`.
- Open a connection per operation (`await using`); the pooled connection is cheap. The keep-alive connection in the constructor exists only so in-memory databases survive.

</details>

## Stretch goals (not tested)

- Store tags relationally (`task_tags`) and compare the queries.
- Add `ETag`/`If-Match` to `PUT` and `PATCH` to prevent lost updates.
- Add an `Idempotency-Key` header to `POST /tasks`.
- Make `PATCH` atomic in one SQL statement instead of read-modify-write.
- Replace the hand-written validator with `AddValidation()` + DataAnnotations and compare the error shape.

## Reference solution

`solution/` contains only the files that differ from `starter/` (`TaskValidator.cs`, `SqliteTaskRepository.cs`, `TaskEndpoints.cs`). Try it yourself first; to check your work against it, `cp -r projects/P2-tasks-crud-api/solution/. projects/P2-tasks-crud-api/starter/` (this overwrites your changes).
