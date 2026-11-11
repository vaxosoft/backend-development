# P2: Tasks CRUD API

A JSON REST API for the Task Tracker domain, persisted in SQLite through `sqlx`, with a repository layer, hand-written validation and the shared error envelope.

**Modules exercised:** [02 HTTP server](../../modules/02-http-server/README.md), [03 REST API design](../../modules/03-rest-api-design/README.md), [04 Serialization](../../modules/04-serialization/README.md), [05 Database access](../../modules/05-database-access/README.md).

## Requirements

Task (JSON, `snake_case`):

| Field | Type | Rules |
|-------|------|-------|
| `id` | int | server-assigned |
| `title` | string | required, trimmed, 1-200 characters |
| `status` | string | `todo` (default), `in_progress`, `done` |
| `due_date` | `YYYY-MM-DD` or `null` | strict format and a real calendar date |
| `tags` | string[] | 0-10 tags, each 1-30 characters, lowercase, no duplicates (default `[]`) |
| `owner_id` | int or `null` | not validated (there is no users table in this project) |
| `created_at` | RFC 3339 UTC, whole seconds, e.g. `2030-01-01T09:00:00Z` | server-assigned |

| Endpoint | Success | Notes |
|----------|---------|-------|
| `POST /tasks` | `201` + `Location: /tasks/{id}` + task | body fields above; absent = default |
| `GET /tasks/{id}` | `200` + task | |
| `PUT /tasks/{id}` | `200` + task | full replace: absent optional fields reset to defaults; `id` and `created_at` are kept |
| `PATCH /tasks/{id}` | `200` + task | only `title`, `status`, `due_date`, `tags`; absent = unchanged; `"due_date": null` clears it; `{}` changes nothing; other fields are ignored |
| `DELETE /tasks/{id}` | `204` | |
| `GET /tasks` | `200` `{"items":[...],"total":N,"limit":L,"offset":O}` | see below |

`GET /tasks?status=&tag=&limit=&offset=&sort=`

- `status`: one of the three statuses; `tag`: exact match; empty values (`?status=`) mean "not given".
- `limit`: integer >= 1, default 20, **clamped** to 100; `offset`: integer >= 0, default 0. The response echoes the effective `limit`; `total` counts all matches, not just the page.
- `sort`: `created_at`, `-created_at` (default), `due_date`, `-due_date`. Ties are broken by `id` in the same direction. Tasks **without** a due date come last in both due-date sorts.

Errors always use the envelope `{"error":{"code","message","fields"?}}` with `Content-Type: application/json`:

| Situation | Status | `code` |
|-----------|--------|--------|
| body is not valid JSON, has wrong types, or is not JSON at all; bad query value (`limit=abc`, `sort=title`, ...) | 400 | `bad_request` |
| well-formed JSON with invalid values (empty title, unknown status, bad date, bad tags) | 422 | `validation_failed` + `fields` (field name -> message; **all** problems at once) |
| unknown id, or an id that is not a positive integer | 404 | `not_found` |
| database failure | 500 | `internal` (details are logged, never sent) |

Storage: SQLite; the schema is created at startup (`CREATE TABLE IF NOT EXISTS`). Tags live in one JSON-array column and are filtered with `json_each`.

## Acceptance criteria (test names in `starter/tests/tasks_api.rs`)

| Criterion | Tests |
|-----------|-------|
| Create: status, `Location`, defaults, trimmed title, all fields kept | `create_returns_201_location_and_the_stored_task`, `create_trims_the_title`, `create_keeps_all_given_fields` |
| Validation -> 422 with `fields` | `create_without_title_is_422_with_a_title_field_error`, `create_enforces_title_length_between_1_and_200`, `create_with_unknown_status_is_422`, `create_with_malformed_due_date_is_422`, `create_rejects_invalid_tag_lists_with_422`, `create_reports_every_invalid_field_at_once` |
| Malformed JSON -> 400 envelope | `malformed_json_is_400_bad_request` |
| Read, unknown id -> 404 | `get_returns_the_created_task`, `get_unknown_or_non_numeric_id_is_404` |
| Full replace | `put_replaces_everything_but_id_and_created_at`, `put_with_invalid_body_is_422_and_changes_nothing`, `put_and_patch_on_unknown_id_are_404` |
| Partial update | `patch_changes_only_the_fields_that_are_present`, `patch_with_null_due_date_clears_it`, `patch_with_invalid_value_is_422_and_changes_nothing` |
| Delete | `delete_returns_204_then_the_task_is_gone` |
| List envelope, order, filters, paging, sorting, limits | `list_uses_the_envelope_and_defaults_to_newest_first`, `list_filters_by_status_and_by_tag`, `list_paginates_and_reports_the_total_of_all_matches`, `list_clamps_limit_to_100`, `list_sorts_by_due_date_with_undated_tasks_last`, `list_with_bad_query_values_is_400` |

## How to run

Work in `projects/P2-tasks-crud-api/starter/` (commands from `languages/rust/`):

```bash
cargo test -p p2-tasks-crud-api        # starter: 25 tests fail until the TODOs are done
ADDR=127.0.0.1:8080 cargo run -p p2-tasks-crud-api
DATABASE_URL='sqlite://tasks.db?mode=rwc' cargo run -p p2-tasks-crud-api   # keep data in a file
```

`ADDR` defaults to `127.0.0.1:8080`, `DATABASE_URL` to `sqlite::memory:` (data is lost on exit). Try it:

```bash
curl -i -X POST localhost:8080/tasks -H 'content-type: application/json' \
     -d '{"title":"Write docs","tags":["docs","p1"],"due_date":"2030-01-31"}'
curl 'localhost:8080/tasks?tag=docs&sort=due_date'
curl -X PATCH localhost:8080/tasks/1 -H 'content-type: application/json' -d '{"status":"done","due_date":null}'
```

## Where the TODOs are

| File | Work |
|------|------|
| `src/validate.rs` | `validate_new`, `validate_patch`, `apply_patch`, `parse_list`, `parse_id` |
| `src/extract.rs` | `ApiJson`: map JSON rejections to the envelope |
| `src/repo.rs` | `create`, `get`, `replace`, `delete`, `list` (SQL) |
| `src/handlers.rs` | six handlers (currently answer `501`) |

Already provided: `error.rs` (`ApiError` and the envelope), `model.rs` (types, including the `Option<Option<_>>` helper for PATCH), the schema/pool setup in `repo.rs`, the router in `lib.rs`.

<details>
<summary>Hints</summary>

- Work bottom-up: `repo.rs` -> `validate.rs` -> `extract.rs` -> `handlers.rs`; run `cargo test -p p2-tasks-crud-api create_` as you go.
- Validation: write one small `fn(&str) -> Result<T, &'static str>` per field, run them all, then `match` the tuple of results. `ApiError::validation([("title", title.err()), ...])` keeps only the failures.
- `RawTask` keeps `status` and `due_date` as `String` on purpose: a bad *value* is a 422, while a wrong JSON *type* is rejected by serde (400).
- Date check: `NaiveDate::parse_from_str(s, "%Y-%m-%d")` also accepts `2030-1-5`; check `s.len() == 10` too.
- `INSERT ... RETURNING *` and `UPDATE ... RETURNING *` return the row; use `fetch_one` / `fetch_optional` with `sqlx::query_as::<_, TaskRow>` and `TaskRow::into_task`. Bind `Utc::now().trunc_subsecs(0)` (`chrono::SubsecRound`) for `created_at`.
- sqlx 0.9 accepts only `&'static str` SQL. For the dynamic `ORDER BY` in `list`, build the string from fixed fragments and wrap it in `sqlx::AssertSqlSafe(..)`. Never interpolate user input.
- Optional filters in one query: `WHERE (?1 IS NULL OR status = ?1)`; bind `Option<&str>`.
- Tag filter: `EXISTS (SELECT 1 FROM json_each(tasks.tags) WHERE json_each.value = ?2)`.
- Undated tasks last: `ORDER BY due_date IS NULL, due_date ASC, id ASC`.
- `ApiJson`: `Json::<T>::from_request(req, state).await` and map the `Err`.
- Handler argument order: `State`, `Path`, then the body extractor **last**. Return `Result<_, ApiError>`; `?` converts `sqlx::Error` for you.
- PATCH: `NewTask::from(task)` gives an editable copy; `apply_patch` it, then `repo.replace`.

</details>

## Stretch goals (not tested)

- Hide `TaskRepo` behind a trait and make the handlers generic over it; add an in-memory fake.
- Move the schema into `migrations/` and use `sqlx::migrate!`.
- Store tags relationally (`task_tags` table) and compare the queries.
- Optimistic concurrency with `ETag` / `If-Match` on `PUT`.
- Run against PostgreSQL by swapping the pool and placeholders (`$1`, ...).

## Reference solution

`solution/` contains only the files that differ from `starter/`, at the same relative paths (`src/validate.rs`, `src/extract.rs`, `src/repo.rs`, `src/handlers.rs`). Try on your own first.
