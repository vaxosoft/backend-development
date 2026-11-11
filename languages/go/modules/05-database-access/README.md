# 05. Database Access in Go

> Estimated time: 2h | Difficulty: Intermediate | Prerequisites: 01, 04

## Learning Outcomes
After this module you can:
- [ ] Open a `database/sql` pool, run parameterised statements and iterate rows without leaking connections
- [ ] Map rows to structs, including `NULL` columns, JSON columns and text timestamps
- [ ] Wrap multi-statement writes in a transaction that rolls back on any error
- [ ] Write versioned migrations and a repository behind a small interface, with sentinel errors
- [ ] Explain what changes when SQLite is swapped for PostgreSQL

## 1. Concept Overview
A service talks to its database through a **pool** of connections. Each query borrows a connection, sends SQL plus separately transmitted **parameters** (never string-concatenated values, which is how SQL injection happens), and returns it. Work that must succeed or fail as a unit runs in a **transaction**. Schema changes are recorded as ordered **migrations** so every environment reaches the same shape, and the SQL is hidden behind a **repository** so handlers and tests depend on behaviour, not on SQL.

## 2. How Go Approaches It

| Need | Tool |
|------|------|
| Portable API | `database/sql`: `*sql.DB` is a concurrency-safe **pool**, not a connection |
| Driver | `modernc.org/sqlite` (pure Go, no cgo, driver name `"sqlite"`); `pgx/v5/stdlib` (`"pgx"`) for PostgreSQL |
| Queries | `QueryContext` (many rows), `QueryRowContext` + `Scan` (one row), `ExecContext` (no rows) |
| NULLs | pointers (`*string`), `sql.Null[T]`, `sql.NullString` and friends |
| Transactions | `db.BeginTx(ctx, opts)` -> `*sql.Tx`; `defer tx.Rollback()` then `Commit()` |
| Pool tuning | `SetMaxOpenConns`, `SetMaxIdleConns`, `SetConnMaxLifetime`, `db.Stats()` |

Idioms: no ORM and no query builder here (both are out of scope for this track); write SQL, scan into plain structs, return `ErrNotFound` for `sql.ErrNoRows`, pass `ctx` to everything, and define the repository **interface at the consumer** rather than next to the implementation. Examples use in-memory SQLite so they need no server.

## 3. Guided Examples

### 3.1 Query basics
- **Goal:** `Exec`, `QueryRow`, `Query`, parameters, prepared statements, pool stats.
- **Run:** `go run ./modules/05-database-access/examples/01_query_basics`
- **Code:** [examples/01_query_basics/main.go](examples/01_query_basics/main.go)
  ```go
  rows, err := db.QueryContext(ctx, `SELECT id, title, status FROM tasks ORDER BY id`)
  if err != nil { return err }
  defer rows.Close()
  for rows.Next() { /* rows.Scan(...) */ }
  if err := rows.Err(); err != nil { return err }
  ```
- **Walkthrough:** `db.SetMaxOpenConns(1)` is required for `":memory:"` because each connection is a separate empty database. A hostile string passed as a `?` argument is only data (0 matches). `QueryRowContext` postpones its error to `Scan`; no row gives `sql.ErrNoRows`.
- **Expected output:**
  ```
  inserted 1
  inserted 2
  inserted 3
  updated 1
  Fix bug <nil>
  true
  1 Write docs todo
  2 Fix bug    done
  3 Ship it    todo
  matches: 0
  todo 2
  done 1
  open connections: 1 max: 1
  ```

### 3.2 NULLs and column mapping
- **Goal:** read and write nullable columns, JSON text and RFC 3339 timestamps.
- **Run:** `go run ./modules/05-database-access/examples/02_null_handling`
- **Code:** [examples/02_null_handling/main.go](examples/02_null_handling/main.go)
- **Walkthrough:** `*string` and `sql.Null[int64]` both scan `NULL` safely and are written back as `NULL` when nil/invalid. Scanning `NULL` into a plain `string` is an error, not `""`. SQLite has no array or timestamp type, so tags are a JSON string and `created_at` is RFC 3339 text parsed with `time.Parse`. In SQL, `= NULL` never matches; use `IS NULL`.
- **Expected output:**
  ```
  scan NULL into string: sql: Scan error on column index 0, name "due_date": converting NULL to string is unsupported
  1 "With everything" due=2030-01-31 owner=7 tags=[docs p1] at=2030-01-01T09:00:00Z
  2 "Mostly empty" due=null owner=null tags=[] at=2030-01-01T09:00:00Z
  owner_id = NULL: 0
  owner_id IS NULL: 1
  ```

### 3.3 Transactions
- **Goal:** all-or-nothing writes with a reusable helper.
- **Run:** `go run ./modules/05-database-access/examples/03_transactions`
- **Code:** [examples/03_transactions/main.go](examples/03_transactions/main.go)
  ```go
  tx, err := db.BeginTx(ctx, opts)
  if err != nil { return err }
  defer tx.Rollback() // harmless after Commit
  if err := fn(tx); err != nil { return err }
  return tx.Commit()
  ```
- **Walkthrough:** the first call moves tasks and deletes a user atomically. The second runs a `DELETE`, fails, and the deferred `Rollback` restores the rows. The third has a `UNIQUE` violation on its second insert, so the first insert disappears too. Foreign keys are only enforced because the DSN enables `_pragma=foreign_keys(1)`. `TxOptions` isolation is a request to the driver (PostgreSQL honours it). The last case calls `db` inside an open transaction: with a pool of one it blocks until the context expires.
- **Expected output:**
  ```
  start:    task1->user1 task2->user1 task3->user2 
  commit:   <nil> task1->user2 task2->user2 task3->user2 
  rollback: simulated failure after the first statement task1->user2 task2->user2 task3->user2 
  unique:   true users: 1
  fk:       constraint failed: FOREIGN KEY constraint failed (787)
  serializable read-only: <nil>
  db inside tx: context deadline exceeded
  ```

### 3.4 Migrations and a repository (Task Tracker)
- **Goal:** the shape of a real data layer: versioned schema, `TaskStore` interface, `Repo` implementation.
- **Run:** `go run ./modules/05-database-access/examples/04_repository_migrations`
- **Code:** [examples/04_repository_migrations/main.go](examples/04_repository_migrations/main.go)
- **Walkthrough:** `migrate` records versions in `schema_migrations` and applies each migration in its own transaction, so a second run applies nothing. `Repo.List` builds `WHERE` from fixed fragments and passes every value as an argument. `ErrNotFound` hides `sql.ErrNoRows` and zero `RowsAffected`; the clock is injected so output is deterministic. Tags are a JSON column queried with SQLite's `json_each` (PostgreSQL would use `jsonb` or `text[]`). `CHECK` constraints are the last line of defence behind validation.
- **Expected output:**
  ```
  migrations applied: [1 2] <nil>
  migrations applied: [] <nil>
  created 1 "Write docs"
  created 2 "Fix bug"
  created 3 "Ship it"
  get 1: "Write docs" due=2030-01-31 tags=[docs p1] at=2030-01-01T09:00:00Z <nil>
  get 42: task not found true
  list {Status: Tag: Limit:0 Offset:0} -> [Write docs Fix bug Ship it]
  list {Status:done Tag: Limit:0 Offset:0} -> [Fix bug]
  list {Status: Tag:p1 Limit:0 Offset:0} -> [Write docs Fix bug]
  list {Status:todo Tag:p1 Limit:0 Offset:0} -> [Write docs]
  list {Status: Tag: Limit:1 Offset:2} -> [Ship it]
  delete 3: <nil>
  delete 3 again: task not found
  empty title: insert task: constraint failed: CHECK constraint failed: length(title) BETWEEN 1 AND 200 (275)
  ```

### 3.5 PostgreSQL as a driver swap
- **Goal:** show what changes; optional, needs a server.
- **Run:** `go run ./modules/05-database-access/examples/05_postgres_swap` (set `DATABASE_URL=postgres://user:pass@localhost:5432/db?sslmode=disable` to use a real server)
- **Code:** [examples/05_postgres_swap/main.go](examples/05_postgres_swap/main.go)
- **Walkthrough:** only the import (`pgx/v5/stdlib`), the driver name (`"pgx"`) and the SQL dialect change: `$1` placeholders, `RETURNING id` instead of `LastInsertId`, identity columns, real `DATE`/`TIMESTAMPTZ`/`TEXT[]` types. Without `DATABASE_URL` it compiles and prints a skip message. The PostgreSQL path is not run by any automated test.
- **Expected output (no `DATABASE_URL`):**
  ```
  DATABASE_URL not set; skipping PostgreSQL demo
  ```

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| Tables "vanish" with `:memory:` | Every pooled connection opens its own empty in-memory DB | `db.SetMaxOpenConns(1)` (or a file DB) |
| Pool exhausted, requests hang | `rows` not closed on an early `return`; `Query` used for statements that return no rows | `defer rows.Close()` right after the error check; use `Exec` |
| Silent truncated results | Loop ended on an I/O error and `rows.Err()` was never checked | Always `return rows.Err()` after the loop |
| `converting NULL to string is unsupported` | Scanning a nullable column into a non-pointer | `*string`, `sql.Null[T]` or `COALESCE` |
| SQL injection | `fmt.Sprintf("... '%s'", name)` | `?` / `$1` arguments; only splice fixed fragments you wrote |
| Deadlock inside a transaction | Calling `db.QueryContext` instead of `tx.QueryContext` needs a second connection | Use the `tx` for everything between `Begin` and `Commit` |
| Half-applied writes | Several `db.Exec` calls with no transaction | `BeginTx` + `defer tx.Rollback()` + `Commit` |
| Foreign keys not enforced | SQLite has them off by default | `_pragma=foreign_keys(1)` in the DSN |
| `sql.ErrNoRows` leaks to handlers | Repository returns driver errors unchanged | Map to a domain `ErrNotFound`, compare with `errors.Is` |
| Unbounded pool under load | Defaults allow unlimited open connections | `SetMaxOpenConns`, `SetConnMaxLifetime`; watch `db.Stats()` |

## 5. Coming From Other Languages
- **Java/C# (JDBC, EF/Hibernate):** `*sql.DB` is the `DataSource`/pool, `*sql.Tx` the explicit transaction; there is no unit-of-work, change tracking or lazy loading. Mapping is manual `Scan`.
- **Python (SQLAlchemy/DB-API):** like a raw cursor, but with a built-in pool and `context.Context` for cancellation instead of connection-level timeouts.
- **Rust (sqlx):** similar explicitness; sqlx can check queries at compile time, Go's `database/sql` checks nothing until runtime, so integration tests against a real DB matter.

## 6. Exercises
Starter code and tests are in [exercises/](exercises/); reference solutions in [_solutions/](_solutions/).

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Query rows | Easy | `Query`/`QueryRow`, NULL mapping, parameters, `rows.Close`/`Err` | `go test ./modules/05-database-access/exercises/ex1_query_rows/` |
| 2 | Atomic create | Medium | transactions, rollback, error wrapping | `go test ./modules/05-database-access/exercises/ex2_transactions/` |
| 3 | Migration runner | Stretch | versioned migrations, per-migration transactions, idempotence | `go test ./modules/05-database-access/exercises/ex3_migrations/` |

### Exercise 1: Query rows
- **Task:** implement `Get`, `List` and `SearchTitle` against the `tasks` table; map NULL `due_date`/`owner_id` to `nil`; return `ErrNotFound`.
- **Acceptance:** `TestGetMapsNulls`, `TestGetNotFound`, `TestList`, `TestListHonoursCancellation`, `TestSearchTitleIsInjectionSafe`, `TestConnectionsAreReleased`.
- <details><summary>Hint</summary>Scan into `*string` / `*int64` fields; share one scan function between `*sql.Row` and `*sql.Rows` through a one-method interface; `LIKE '%' || ? || '%'` keeps the term a parameter.</details>

### Exercise 2: Atomic create
- **Task:** `CreateWithTags` currently inserts without a transaction, so a failure leaves an orphan task. Make it atomic.
- **Acceptance:** `TestCreateWithTagsIsAtomic`, `TestPoolIsUsableAfterFailure`, `TestCancelledContextStoresNothing`, plus the happy path.
- <details><summary>Hint</summary>`defer tx.Rollback()` immediately after `BeginTx`, use `tx` for every statement, `Commit` last.</details>

### Exercise 3: Migration runner
- **Task:** implement `Migrate` and `Version`: sorted, idempotent, one transaction per migration, duplicate versions rejected.
- **Acceptance:** all tests in `migrate_test.go`, notably `TestFailingMigrationRollsBackOnlyItself`.
- <details><summary>Hint</summary>Check duplicates before touching the database; put the migration SQL and its `schema_migrations` insert in the same `tx`; a multi-statement string works with one `ExecContext` call on SQLite.</details>

## 7. Recap
- `*sql.DB` is a pool: create it once, close rows, check `rows.Err()`, and pass `ctx` everywhere.
- Parameters for values, fixed fragments for structure; never concatenate user input.
- `BeginTx` + `defer Rollback` + `Commit`, and use the `tx` inside it.
- Hide SQL behind a repository interface with domain errors; migrate with recorded versions.
- Reach for plain `database/sql` first; add a query builder or sqlc only when the SQL volume justifies it.

## 8. Self-check Questions
1. Why does a `:memory:` database lose its tables with the default pool settings, and what are two ways to fix it?
2. What happens to the connection if you return early from a `for rows.Next()` loop without `defer rows.Close()`?
3. Why is `defer tx.Rollback()` safe to leave in place after a successful `Commit`?
4. How do you tell "no row" from "database error" in `QueryRowContext(...).Scan`, and where should that translation live?
5. Which three things differ in the code when moving the repository to PostgreSQL?

## 9. Further Reading
- [Accessing relational databases](https://go.dev/doc/database/) (Go docs)
- [database/sql](https://pkg.go.dev/database/sql) package documentation
- [pgx](https://pkg.go.dev/github.com/jackc/pgx/v5) documentation

---
**Next:** [06 Concurrency](../06-concurrency/README.md)
