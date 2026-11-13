# P2: Tasks CRUD API

**Modules exercised:** [02 HTTP server](../../modules/02-http-server/README.md), [03 REST API design](../../modules/03-rest-api-design/README.md), [04 Serialization](../../modules/04-serialization/README.md), [05 Database access](../../modules/05-database-access/README.md).

## Goal

A JSON CRUD API for the Task Tracker domain: Pydantic validation, the shared error envelope, a repository abstraction and SQLite persistence.

## Requirements

### Endpoints

| Method and path | Success | Notes |
|-----------------|---------|-------|
| `POST /tasks` | `201` + `Location: /tasks/{id}` + task | server assigns `id` and `created_at` |
| `GET /tasks/{id}` | `200` task | unknown or non-numeric id: `404` |
| `PUT /tasks/{id}` | `200` task | full replace: absent optional fields reset to their defaults; `id` and `created_at` stay |
| `PATCH /tasks/{id}` | `200` task | only `title`, `status`, `due_date`, `tags` are patchable; absent = unchanged; `"due_date": null` clears it; other keys are ignored |
| `DELETE /tasks/{id}` | `204`, no body | unknown id: `404` |
| `GET /tasks` | `200` `{"items":[...],"total":N,"limit":L,"offset":O}` | see below |

`GET /tasks?status=&tag=&limit=&offset=&sort=`:

- `status`: `todo`, `in_progress` or `done`; `tag`: exact tag match.
- `limit` default `20`, values above `100` are clamped to `100`; `offset` default `0`; `total` counts all matches, not just the page.
- `sort`: `created_at`, `-created_at` (default), `due_date`, `-due_date`. Tasks without a due date always come last. Ties are broken by `id` in the same direction.
- `limit=abc`, `limit=0`, `offset=-1`, an unknown `status` or `sort` are `400 bad_request`.

### Task rules

| Field | Rule |
|-------|------|
| `title` | required string, trimmed, 1-200 characters |
| `status` | `todo` (default), `in_progress`, `done` |
| `due_date` | `null` or exactly `YYYY-MM-DD` and a real calendar date |
| `tags` | 0-10 tags, each 1-30 characters, lowercase, no duplicates |
| `owner_id` | integer or `null` |
| `created_at` | RFC 3339 UTC, for example `2030-01-01T09:00:00Z`, read-only |

### Errors

Every error is `{"error":{"code","message","fields"?}}` with `Content-Type: application/json`:

| Situation | Status | `code` |
|-----------|--------|--------|
| malformed JSON, bad query value | 400 | `bad_request` |
| unknown id (also `/tasks/abc`) | 404 | `not_found` |
| invalid task data | 422 | `validation_failed` with `fields` = `{"title": "...message..."}` (one entry per invalid field) |

Persistence: SQLite, schema created at startup, data survives a restart on the same file. `create_app(Config(db_path=..., clock=...))` is the entry point; the clock is injectable so tests control `created_at`.

## Run

```bash
python -m pytest projects/P2-tasks-crud-api/tests          # from languages/python
cd projects/P2-tasks-crud-api/starter
ADDR=127.0.0.1:8080 DB_PATH=tasks.db python -m p2_tasks_crud_api   # defaults: 127.0.0.1:8080, tasks.db
```

```bash
curl -i -X POST localhost:8080/tasks -H 'content-type: application/json' \
  -d '{"title":"Write docs","tags":["docs"],"due_date":"2030-01-31"}'
curl -s -X PATCH localhost:8080/tasks/1 -H 'content-type: application/json' -d '{"status":"done"}'
curl -s 'localhost:8080/tasks?tag=docs&sort=due_date&limit=5'
```

## Acceptance criteria (tests in `tests/test_p2_tasks.py`)

| Criterion | Tests |
|-----------|-------|
| create, `Location`, defaults, ids | `test_create_returns_201_location_and_defaults`, `test_create_stores_every_field_and_trims_the_title`, `test_create_assigns_distinct_ids` |
| validation (422 + field names), limits | `test_create_rejects_invalid_data_with_422_and_the_field_name`, `test_create_accepts_the_limits` |
| malformed JSON | `test_malformed_json_is_400_bad_request` |
| read, 404 | `test_get_returns_the_task`, `test_unknown_or_non_numeric_id_is_404` |
| full replace | `test_put_replaces_everything_but_id_and_created_at`, `test_put_validates_and_404s_on_unknown_id` |
| partial update | `test_patch_changes_only_the_given_fields`, `test_patch_null_due_date_clears_it`, `test_patch_updates_title_and_tags_and_ignores_read_only_fields`, `test_patch_validates_the_result`, `test_patch_unknown_id_is_404` |
| delete | `test_delete_returns_204_and_the_task_is_gone`, `test_delete_unknown_id_is_404` |
| list envelope, sort, filters, paging | `test_list_returns_the_envelope_with_defaults`, `test_list_defaults_to_newest_first_and_sorts_by_created_at`, `test_list_sorts_by_due_date_with_missing_dates_last`, `test_list_filters_by_status_and_tag`, `test_list_paginates_and_reports_the_total_of_all_matches`, `test_list_clamps_limit_to_100`, `test_list_rejects_bad_query_values_with_400` |
| SQLite persistence | `test_tasks_survive_a_restart_on_the_same_database_file` |

## What to implement

The starter has the wiring, the `Task` output model, the schema, row mapping and routing signatures. Every `# TODO` is yours:

| File | Work |
|------|------|
| `starter/p2_tasks_crud_api/models.py` | validation rules of `TaskIn` |
| `starter/p2_tasks_crud_api/repository.py` | the five SQL methods of `SqliteTaskRepository` |
| `starter/p2_tasks_crud_api/app.py` | route bodies |
| `starter/p2_tasks_crud_api/errors.py` | validation exception handlers that build the envelope |

<details>
<summary>Hints</summary>

- `Annotated[str, StringConstraints(strip_whitespace=True, min_length=1, max_length=200)]` covers the title; `Field(max_length=10)` caps the list; an `AfterValidator` on the item type checks lowercase; a `field_validator("tags")` checks duplicates.
- `date.fromisoformat` also accepts `20300131`: add a `field_validator("due_date", mode="before")` that requires `\d{4}-\d{2}-\d{2}` first.
- Pydantic's `msg` for a failed validator starts with `Value error, `; strip it for the `fields` message.
- A `RequestValidationError` has `errors()` entries with `loc` such as `("body", "title")` or `("query", "limit")` and `type == "json_invalid"` for malformed JSON.
- PATCH: `current.model_dump(mode="json", include=...)`, update with the patchable keys of the body, then `TaskIn.model_validate(merged)`; a `pydantic.ValidationError` raised there needs its own handler.
- SQL: `cursor.lastrowid`, `cursor.rowcount`, `with self._db:` commits or rolls back. Never format user input into SQL; map `sort` through a dict of fixed `ORDER BY` clauses. `json_each(tasks.tags)` turns the JSON array into rows.
- Handlers are plain `def`: FastAPI runs them in a thread pool, so blocking `sqlite3` calls do not stall the event loop (the repository's lock protects the shared connection).

</details>

## Stretch goals (not tested)

- Add `GET /openapi.json` examples/descriptions through `response_model` and `responses=`.
- Replace the JSON `tags` column with a `task_tags` table and a migration step.
- Add `ETag`/`If-Match` on `PUT` to prevent lost updates.
- Run the same repository tests against a second implementation (an in-memory dict) to prove the abstraction.

## Reference solution

[`solution/`](./solution/) contains the four files that differ from the starter (`models.py`, `repository.py`, `app.py`, `errors.py`). Try the project first, then compare.
