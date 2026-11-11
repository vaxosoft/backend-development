# P2 - Tasks CRUD API

> **Modules exercised:** [02 HTTP Server](../../modules/02-http-server/README.md), [03 REST API Design](../../modules/03-rest-api-design/README.md), [04 Serialization](../../modules/04-serialization/README.md), [05 Database Access](../../modules/05-database-access/README.md)
> **Time:** 3-4 hours · **Difficulty:** Intermediate

## Goal

Build a JSON CRUD API for the Task Tracker domain on `net/http` and `database/sql` (SQLite), with validation, a repository abstraction, filtering, sorting and pagination.

## Requirements

Wire format, validation rules and error envelope follow [domain-and-projects.md](../../../../framework/domain-and-projects.md) (sections 1-2). A task:

```json
{"id":1,"title":"Write docs","status":"todo","due_date":"2030-01-31","tags":["docs","p1"],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}
```

| Endpoint | Behaviour |
|----------|-----------|
| `POST /tasks` | body = `title` (required), `status`, `due_date`, `tags`, `owner_id`; `201`, `Location: /tasks/{id}`, the created task. `id` and `created_at` in the body are ignored |
| `GET /tasks/{id}` | `200` task; unknown **or non-numeric** id -> `404 not_found` |
| `PUT /tasks/{id}` | full replace: omitted optional fields reset to defaults (`status` `todo`, `due_date`/`owner_id` `null`, `tags` `[]`); `id` and `created_at` kept; `200` |
| `PATCH /tasks/{id}` | only `title`, `status`, `due_date`, `tags` may change (other keys, such as `owner_id`, are ignored); an absent key is unchanged; `"due_date": null` clears the date; the **merged** result is validated; `200` |
| `DELETE /tasks/{id}` | `204`, no body |
| `GET /tasks` | `?status=&tag=&limit=&offset=&sort=` -> `{"items":[...],"total":N,"limit":L,"offset":O}`; `total` counts all matches, not just the page |

Details to get right:

- **Validation (`422 validation_failed` + `fields`)**: title 1-200 characters after trimming (count characters, not bytes); `status` in `todo|in_progress|done` (default `todo`); `due_date` a real `YYYY-MM-DD` date; at most 10 tags, each 1-30 characters, lowercase, no duplicates. All problems are reported at once, keyed by JSON field name.
- **`400 bad_request`**: invalid JSON, empty body, a body of the wrong JSON type, and bad query values (`status` not in the enum, `limit`/`offset` not integers, `limit < 1`, `offset < 0`, unknown `sort`).
- **`413 payload_too_large`**: request bodies above 1 MiB.
- **Lists**: `limit` default 20, values above 100 are clamped to 100, `offset` default 0. `sort` is `created_at`, `-created_at` (default), `due_date` or `-due_date`; tasks without a due date sort last in both directions; ties break by `id` so the order is deterministic. `items` is `[]`, never `null`. `tag` matches tasks that contain that exact tag.
- Every response, errors included, is `application/json`. A `500` never leaks internals.
- Persistence in SQLite behind the `Store` interface; the schema is created in `Migrate` at startup. Tags are stored as a JSON array in a `TEXT` column (filter with `json_each`).

## Layout

```
projects/P2-tasks-crud-api/
├── starter/            package starter (+ cmd/server/main.go, complete)
│   ├── app.go          OpenDB, New(db) wiring and routes - complete
│   ├── task.go         Task, TaskInput given; TODO Validate
│   ├── store.go        Store, ListQuery, orderBy given; TODO SQLiteStore methods
│   └── handlers.go     helpers given; TODO decodeBody, taskID, parseListQuery, six handlers
├── _solution/           task.go, store.go, handlers.go (overlay)
└── tests/              black-box tests over real HTTP, SQLite in memory (never overlaid)
```

## Acceptance criteria

| Criterion | Tests |
|-----------|-------|
| Create: 201, `Location`, defaults, explicit `[]`/`null`, timestamp | `TestCreateReturns201WithLocationAndDefaults`, `TestCreateStoresAllFieldsAndTimestampIsRecentUTC`, `TestCreateAssignsIncreasingIDsAndTrimsTitle` |
| Validation rules -> 422 + fields | `TestCreateRejectsInvalidDataWith422AndFieldMessages`, `TestCreateAcceptsBoundaryValues` |
| Malformed JSON -> 400, oversized -> 413 | `TestMalformedJSONIs400BadRequest`, `TestOversizedBodyIs413` |
| Read, unknown id -> 404 | `TestGetReturnsTheStoredTask`, `TestUnknownOrMalformedIDIs404WithEnvelope` |
| Delete | `TestDeleteReturns204ThenTaskIsGone` |
| PUT replaces | `TestPutReplacesEveryWritableField`, `TestPutInvalidDataIs422AndLeavesTaskUnchanged` |
| PATCH semantics | `TestPatchChangesOnlyTheGivenFields`, `TestPatchNullDueDateClearsIt`, `TestPatchIgnoresOwnerID`, `TestPatchValidatesTheMergedResult`, `TestPatchMalformedBodyIs400` |
| Unknown id on writes | `TestWritingToAnUnknownIDIs404ForPutPatchAndDelete` |
| List envelope and defaults | `TestListOnEmptyStoreHasEmptyItemsArrayAndDefaults` |
| Sorting | `TestListDefaultsToNewestFirstAndSortCanReverseIt`, `TestListSortsByDueDateWithMissingDatesLast` |
| Filtering | `TestListFiltersByStatusAndTag`, `TestSQLInjectionInFiltersIsHarmless` |
| Pagination, clamp, bad values | `TestListPaginatesAndReportsTotal`, `TestListClampsLimitTo100`, `TestListRejectsBadQueryValuesWith400` |

## How to run

From `languages/go/`:

```bash
ADDR=127.0.0.1:8080 DB_PATH=tasks.db go run ./projects/P2-tasks-crud-api/starter/cmd/server
go test ./projects/P2-tasks-crud-api/tests/            # fails until you are done
go test -race -count=1 ./projects/P2-tasks-crud-api/tests/
```

`ADDR` defaults to `127.0.0.1:8080`, `DB_PATH` to `tasks.db` (created on first start). Try it (`created_at` differs on your machine):

```bash
curl -si -X POST localhost:8080/tasks -d '{"title":"Write docs","tags":["docs","p1"],"due_date":"2030-01-31"}'
# HTTP/1.1 201 Created
# Location: /tasks/1
# {"id":1,"title":"Write docs","status":"todo","due_date":"2030-01-31","tags":["docs","p1"],"owner_id":null,"created_at":"..."}

curl -s -X PATCH localhost:8080/tasks/1 -d '{"status":"in_progress","due_date":null}'
# {"id":1,"title":"Write docs","status":"in_progress","due_date":null,"tags":["docs","p1"],...}

curl -s -X POST localhost:8080/tasks -d '{"title":"","tags":["Bad"]}'
# {"error":{"code":"validation_failed","message":"invalid task","fields":{"tags":"tag \"Bad\" must be lowercase","title":"must be 1-200 characters"}}}

curl -s 'localhost:8080/tasks?tag=docs&limit=5'
# {"items":[{...}],"total":1,"limit":5,"offset":0}
```

## Suggested steps

1. `Validate` (pure function, no HTTP yet) and `decodeBody`/`taskID`.
2. `Migrate`, `Create`, `Get`; then `create` and `get` handlers. The first tests should pass.
3. `Replace`, `Delete` and the `replace`/`delete` handlers.
4. `parseListQuery`, `List` and the `list` handler.
5. `patch` last: it is the only handler that must tell "absent" from "null".

<details>
<summary>Hints</summary>

- `ExecContext(..., in.DueDate, in.OwnerID)`: a nil `*string`/`*int64` is stored as SQL `NULL`; scan back through `sql.NullString` / `sql.NullInt64`.
- Store `created_at` as `time.Now().UTC().Truncate(time.Second).Format(time.RFC3339)` and parse it with `time.Parse(time.RFC3339, ...)`.
- `res.LastInsertId()` gives the new id; `res.RowsAffected() == 0` means "no such task".
- `OpenDB` pins one connection (each `:memory:` connection is a separate empty database). Consequence: finish reading `rows` (and `defer rows.Close()`) before issuing another query, and run the `COUNT(*)` first.
- Never concatenate request values into SQL: bind them with `?`; for `ORDER BY` look the value up in `orderBy` and concatenate only the fixed string.
- Tag filter: `EXISTS (SELECT 1 FROM json_each(tasks.tags) WHERE value = ?)`.
- Collect `WHERE` fragments in a slice and their arguments in a parallel `[]any`; both the count and the page query reuse them.
- `PATCH`: decode into `map[string]json.RawMessage`, load the current task, copy it into a `TaskInput`, then `json.Unmarshal` each present key into the matching field (a `null` into a `*string` sets it to `nil`). Run `Validate` on the merged result.
- `errors.As(err, new(*http.MaxBytesError))` detects an oversized body after `http.MaxBytesReader`.
- A `nil` slice marshals to `null`; start lists and tags from `[]T{}`.

</details>

## Stretch goals (not tested)

- Make PATCH atomic with a transaction (`BeginTx`, `defer tx.Rollback()`) so two concurrent patches cannot lose an update.
- Add `If-Match` / a `version` column for optimistic concurrency (`412` on conflict).
- Add `GET /tasks?q=` title search with `LIKE` (escape `%` and `_`).
- Swap SQLite for PostgreSQL using `pgx/v5/stdlib` (see module 05); only `OpenDB`, the DDL and the `?` placeholders (`$1`...) change.

## Reference solution

[`_solution/`](./_solution/) holds `task.go`, `store.go` and `handlers.go`. Copy over the starter to see every test pass:

```bash
cp -r projects/P2-tasks-crud-api/_solution/. projects/P2-tasks-crud-api/starter/
```
