# 05. Database Access in Rust

> Estimated time: 2h | Difficulty: Intermediate | Prerequisites: 01, 04 (serde types), async basics from 02

## Learning Outcomes
After this module you can:
- [ ] Open a pool, run parameterised queries and read rows with `sqlx` (SQLite here, PostgreSQL by swapping the driver)
- [ ] Map rows to structs with `FromRow`, including `NULL` (`Option`), enums and dates
- [ ] Use transactions correctly (commit, rollback, rollback-on-drop) and avoid pool starvation
- [ ] Apply migrations with `sqlx::migrate!` and hide SQL behind a repository trait
- [ ] Build filtered, paginated queries without string-concatenating user input

## 1. Concept Overview
A backend talks to a database through a **connection pool** (a fixed set of reusable connections), sends SQL with **bound parameters** (values travel separately from the SQL text, which prevents injection and lets the database cache the statement), and maps result rows to application types. Multi-step changes run in a **transaction** (all or nothing). Schema changes are versioned **migrations** applied in order. A **repository** hides all of it behind domain-shaped functions so handlers and tests do not know which database is used.

```
handler -> repository (trait) -> pool -> connection -> SQL + bound values -> rows -> structs
```
Isolation levels (how visible concurrent transactions are to each other) are a database setting: SQLite is effectively serializable (one writer at a time); PostgreSQL defaults to READ COMMITTED and lets you choose per transaction (example 3.4). An optional key-value store (Redis) is out of scope here.

## 2. How Rust Approaches It
- **`sqlx` 0.9** (PROFILE choice): async, no ORM, one API for SQLite and PostgreSQL. `Pool<DB>` is built in and is `Clone` (a cheap handle), so you put it in axum `State`.
- Queries are **runtime-checked** here (`query`, `query_as`, `query_scalar`); the compile-time-checked `query!` macros need a live database at build time and are not used.
- `#[derive(FromRow)]` maps columns to fields by **name**; `#[derive(sqlx::Type)]` stores an enum as TEXT; `Option<T>` is `NULL`.
- `sqlx 0.9` accepts only `&'static str` as SQL. Building SQL with `format!` does not compile unless you wrap it in `AssertSqlSafe(..)`, a deliberate speed bump. Dynamic `WHERE` clauses use `QueryBuilder` + `push_bind`.
- A `Transaction` rolls back when dropped without `commit()`, so `?` is the rollback path.
- Errors: `sqlx::Error`; a constraint failure is `err.as_database_error()` + `is_unique_violation()`. Wrap it in a `thiserror` enum at the repository boundary.
- Database for examples: SQLite in memory. PostgreSQL is a compile-checked section that only connects if `DATABASE_URL` is set.

## 3. Guided Examples

### 3.1 Connect, execute, query
- **Goal:** pool, bind parameters, rows, `NULL`, "no rows".
- **Run:** `cargo run -p m05-database-access --example connect_and_query`
- **Code:** [examples/connect_and_query.rs](examples/connect_and_query.rs)
```rust
let pool = SqlitePoolOptions::new().max_connections(1).connect("sqlite::memory:").await?;
sqlx::query("INSERT INTO tasks (title, due_date) VALUES (?, ?)").bind("Write docs").bind(due).execute(&pool).await?;
```
- **Walkthrough:** `execute` returns `rows_affected`/`last_insert_rowid`. `Option<&str>` binds `NULL` when `None`. The hostile title is stored as plain text. `fetch_optional` gives `Option<Row>`, `fetch_one` turns "no row" into an error. The last block shows the in-memory pitfall: each `connect("sqlite::memory:")` is its own database, and it disappears when its connections do.
- **Expected output:**
  ```
  rows_affected=1 last_insert_rowid=1
  1 | Write docs | Some("2030-01-31")
  2 | Review PR | None
  3 | x'); DROP TABLE tasks; -- | None
  count=3
  fetch_optional -> is_none=true
  fetch_one -> no rows returned by a query that expected to return at least one row
  another pool -> error returned from database: (code: 1) no such table: tasks
  ```

### 3.2 Rows to types
- **Goal:** `FromRow`, enum as TEXT, `Option`, chrono, decode errors.
- **Run:** `cargo run -p m05-database-access --example rows_to_types`
- **Code:** [examples/rows_to_types.rs](examples/rows_to_types.rs)
```rust
#[derive(Debug, Clone, Copy, sqlx::Type)]
#[sqlx(type_name = "TEXT", rename_all = "snake_case")]
enum Status { Todo, InProgress, Done }
```
- **Walkthrough:** `query_as::<_, Task>` fills the struct by column name. SQLite stores the enum as `in_progress` and the timestamp as RFC 3339 text. Because mapping is checked when the query *runs*, a missing column or an unknown enum value is a runtime `Err`, never a panic.
- **Expected output:**
  ```
  Task { id: 1, title: "Write docs", status: InProgress, due_date: Some(2030-01-31), owner_id: Some(7), created_at: 2030-01-01T09:00:00Z }
  Task { id: 2, title: "Review PR", status: Todo, due_date: None, owner_id: None, created_at: 2030-01-01T09:00:00Z }
  stored as ("in_progress", "2030-01-01T09:00:00+00:00")
  todo: ["Review PR"]
  missing column -> no column found for name: status
  unknown status -> error occurred while decoding column "status": invalid value "archived" for enum Status
  ```

### 3.3 Transactions
- **Goal:** atomic multi-step writes, rollback, pool starvation.
- **Run:** `cargo run -p m05-database-access --example transactions`
- **Code:** [examples/transactions.rs](examples/transactions.rs)
```rust
let mut tx = pool.begin().await?;
let id: i64 = sqlx::query_scalar("INSERT INTO tasks (title) VALUES (?) RETURNING id").bind(title).fetch_one(&mut *tx).await?;
sqlx::query("INSERT INTO audit (task_id, note) VALUES (?, ?)").bind(id).bind(note).execute(&mut *tx).await?;
tx.commit().await?;     // an early `?` above drops `tx` = rollback
```
- **Walkthrough:** the second insert violates a `CHECK`, so the first one disappears too. Statements run on `&mut *tx`, not on `pool`. The last block holds the only connection in a transaction and then uses `pool`: it waits until `acquire_timeout` and fails.
- **Expected output:**
  ```
  committed task 1: tasks=1 audit=1
  failed: error returned from database: (code: 275) CHECK constraint failed: length(note) > 0
  after rollback: tasks=1 audit=1
  inside the transaction: tasks=2
  after rollbacks: tasks=1
  using `pool` inside a transaction -> pool timed out while waiting for an open connection
  after drop: tasks=1
  ```

### 3.4 PostgreSQL swap (compile-only unless `DATABASE_URL` is set)
- **Goal:** see what changes when the database changes.
- **Run:** `cargo run -p m05-database-access --example postgres_swap` (optionally with `DATABASE_URL=postgres://user:pass@localhost/tasks`)
- **Code:** [examples/postgres_swap.rs](examples/postgres_swap.rs)
- **Walkthrough:** `PgPoolOptions`/`PgPool` replace the SQLite types; placeholders become `$1, $2`; `BIGSERIAL`, `DATE`, `TIMESTAMPTZ` replace the SQLite column types; the isolation level is set with `SET TRANSACTION ISOLATION LEVEL ...` as the first statement of a transaction. The automated checks only compile this example; it was not run against a server here.
- **Expected output** (no `DATABASE_URL`):
  ```
  DATABASE_URL is not set: skipping the PostgreSQL demo
  ```

### 3.5 Task repository: migrations, trait, filters, pagination
- **Goal:** the shape you will reuse in the capstones.
- **Run:** `cargo run -p m05-database-access --example task_repository`
- **Code:** [examples/task_repository.rs](examples/task_repository.rs), [migrations/](migrations/)
```rust
static MIGRATOR: sqlx::migrate::Migrator = sqlx::migrate!("./migrations");
trait TaskRepository { async fn list(&self, filter: &TaskFilter) -> Result<Page, RepoError>; /* ... */ }
```
- **Walkthrough:** `migrate!` embeds the `.sql` files at compile time; `run` records each in `_sqlx_migrations`, so the second call does nothing. `list` builds `WHERE 1 = 1 AND status = ? AND EXISTS (...)` with `QueryBuilder`, binds limit/offset (clamped 1-100, default 20) and runs a second `COUNT(*)` with the same filter for `total`. `create` is one transaction (task + tags). `RepoError::NotFound` comes from `rows_affected() == 0`.
- **Expected output:**
  ```
  migrations: [(1, "create tasks"), (2, "create task tags")]
  all: total=4
    #4 Fix bug      Todo due=- tags=["code", "p1"]
    #3 Ship v1      Done due=2030-02-01 tags=["p1", "release"]
    #2 Review PR    InProgress due=- tags=["code"]
    #1 Write docs   Todo due=2030-01-31 tags=["docs", "p1"]
  tag=p1 status=todo: total=2
    #4 Fix bug      Todo due=- tags=["code", "p1"]
    #1 Write docs   Todo due=2030-01-31 tags=["docs", "p1"]
  page 2 of size 2: total=4
    #2 Review PR    InProgress due=- tags=["code"]
    #1 Write docs   Todo due=2030-01-31 tags=["docs", "p1"]
  updated: #2 Review PR    Done due=- tags=["code"]
  after delete: None
  error: task 99 not found
  ```

## 4. Common Pitfalls
| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| Tables "disappear" in tests | every `connect("sqlite::memory:")` is a new database, and it lives only while a connection is open | build one pool, keep `max_connections(1)`, pass it around; use a file for anything long-lived |
| Using `pool` while holding a `Transaction` hangs/times out | the transaction owns a pool connection; with a small pool nobody else can get one | run every statement of the unit of work on `&mut *tx`; keep transactions short |
| `format!("... WHERE tag = '{tag}'")` | looks simpler than `bind` | sqlx 0.9 refuses non-literal SQL; bind values, use `QueryBuilder::push_bind`, and only `AssertSqlSafe` audited fragments (never user input) |
| `fetch_one` on a lookup returns an error for a missing row | "not found" is `RowNotFound` | use `fetch_optional` and map `None` to 404 |
| `NULL` column decoded into `String`/`i64` fails | Rust has no implicit null | `Option<T>` for nullable columns, in structs and with `row.get` |
| Holding a `std::sync::MutexGuard` (or a transaction) across `.await` on shared state | the future becomes non-`Send` or blocks other tasks | scope the guard before `.await`, use `tokio::sync::Mutex` only when you must hold it |
| Editing an applied migration | sqlx stores a checksum per migration | add a new numbered migration instead |
| `?` placeholders on PostgreSQL | SQLite uses `?`, PostgreSQL uses `$1` | write per-database SQL, or keep it in the repository implementation |
| One query per row to load tags (N+1) | simple to write | fine for pages of <= 100 here; otherwise join or `WHERE task_id IN (...)` |

## 5. Coming From Other Languages
- **Go (`database/sql`):** `sqlx::Pool` ~ `*sql.DB`; `bind` ~ `?` args; `rows.Scan` ~ `FromRow`; `defer tx.Rollback()` is automatic here (rollback on drop).
- **Java/C#:** no JPA/EF: SQL stays explicit; `FromRow` ~ a row mapper; `QueryBuilder` ~ a minimal dynamic query builder, not an ORM.
- **Python/TypeScript:** `sqlx` is async like `asyncpg`/`node-postgres`; the pool is shared by cloning a handle instead of passing a global.
- **C++:** the transaction guard is RAII: drop = rollback.

## 6. Exercises
Crates in [exercises/](exercises/), reference solutions in [solutions/](solutions/). All tests use in-memory SQLite.

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | User queries | Easy | `bind`, `query_as`, `fetch_optional`, unique violation | `cargo test -p m05-ex1-user-queries` |
| 2 | Transactions | Medium | `begin`/`commit`, rollback on early return | `cargo test -p m05-ex2-transactions` |
| 3 | Task repository | Stretch | `QueryBuilder`, filters, pagination + total, `rows_affected` | `cargo test -p m05-ex3-task-repository` |

**Exercise 1 - User queries.** *Task:* implement `insert_user` (returns the stored row), `find_by_email`, `list_users`, `count_users`. *Acceptance:* `tests/users.rs` (including the hostile-input test). *Hint:* `INSERT ... RETURNING` plus `query_as(...).fetch_one`.

**Exercise 2 - Transactions.** *Task:* make `create_task_with_tags` and `replace_tags` atomic. *Acceptance:* `tests/transactions.rs`: a duplicate tag must leave no task (create) and keep the old tags (replace). *Hint:* every statement goes to `&mut *tx`; do not call `commit` on the error path.

**Exercise 3 - Task repository.** *Task:* implement `push_filter`, `list` (newest first, `status` and `tag` filters, `limit`/`offset`, `total` of the whole match), `update_status`, `delete`. *Acceptance:* `tests/repository.rs`. *Hint:* build the SELECT and the COUNT from the same `push_filter`; check `rows_affected()` for update/delete.

## 7. Recap
- One shared pool; bind every value; `fetch_optional` for lookups.
- `FromRow` + `Option` + `sqlx::Type` map rows safely; mismatches are runtime errors you should test.
- A transaction is a value: use it for every statement of the unit, commit last, let drop roll back.
- Migrations are append-only files embedded with `migrate!`; repositories keep SQL out of handlers.
- Reach for `query_as` for reads, `QueryBuilder` for dynamic filters, and a trait when callers need a fake.

## 8. Self-check Questions
1. Why does this module use `max_connections(1)` for in-memory SQLite, and what would go wrong with a file database and a bigger pool?
2. A handler begins a transaction, then calls a helper that uses `pool` directly. What happens on a pool of size 1, and on a pool of size 5?
3. How does `list` guarantee that a hostile `tag` value cannot change the SQL?
4. Which errors become 404, 409 and 500 when a repository call fails (`NotFound`, unique violation, anything else)?

## 9. Further Reading
- sqlx docs: <https://docs.rs/sqlx> and the repository README: <https://github.com/launchbadge/sqlx>
- SQLite transactions: <https://www.sqlite.org/lang_transaction.html>; PostgreSQL isolation: <https://www.postgresql.org/docs/current/transaction-iso.html>

---
**Next:** [06 Concurrency](../06-concurrency/README.md)
