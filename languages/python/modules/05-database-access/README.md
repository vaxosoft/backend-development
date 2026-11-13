# 05. Database Access in Python

> Estimated time: 2h | Difficulty: Intermediate | Prerequisites: 01 Language Fundamentals, 04 Serialization

## Learning Outcomes
After this module you can:
- [ ] Open a connection, run parameterised queries and fetch rows with the DB-API (`sqlite3`).
- [ ] Map rows to typed objects and handle `NULL`, dates and missing rows.
- [ ] Use transactions correctly: commit, rollback, atomic multi-statement changes.
- [ ] Apply versioned schema migrations and explain per-connection settings such as foreign keys.
- [ ] Hide SQL behind a repository and test it against a real (in-memory) database.
- [ ] Compare hand-written SQL, a query builder (SQLAlchemy Core) and an ORM, and know how PostgreSQL swaps in.

## 1. Concept Overview
A backend talks to its database through a **driver**: open a connection, send SQL text plus separate **parameters**, read rows, and wrap related writes in a **transaction** so they succeed or fail together. Parameters are never glued into the SQL string; that single rule prevents SQL injection and lets the database reuse query plans.

```
handler --> repository (Task objects) --> connection (SQL + params) --> database
                                              └ transaction: BEGIN ... COMMIT | ROLLBACK
```

Connections are expensive and not always thread-safe, so servers either open one per unit of work or borrow from a **pool**. Schema changes are versioned **migrations** applied once, in order. Isolation levels decide what concurrent transactions can see: SQLite serialises writers, PostgreSQL defaults to `READ COMMITTED` and lets you ask for stricter levels per transaction. (Key-value stores such as Redis are an optional extra and not covered here.)

## 2. How Python Approaches It

| Need | Tool | Notes |
|------|------|-------|
| Driver API | **DB-API 2.0 (PEP 249)**: `connect`, `cursor`/`execute`, `fetchone`/`fetchall`, `commit`, `rollback` | every driver looks like this |
| Embedded database | `sqlite3` (stdlib) | tests, tools and small services; `:memory:` or a file |
| Row mapping | `row_factory = sqlite3.Row`, then build `dataclass` objects | explicit and cheap; no magic |
| Query builder | `SQLAlchemy` 2.x **Core** (`select`, `insert`, `Table`) | composes conditions; pooling via `create_engine`; no ORM sessions here |
| PostgreSQL | `psycopg` 3 | `%s` placeholders; same DB-API rules; only runs when `DATABASE_URL` is set |

Idiomatic Python uses context managers for resources and transactions (`with conn:`, `contextlib.closing`, `engine.begin()`), exceptions for failures (`sqlite3.IntegrityError`) and `None` for "no such row". Non-idiomatic: building SQL with f-strings, leaving connections open, catching `Exception` around a whole transaction. The `sqlite3` module has two transaction modes: the legacy default (it opens a transaction implicitly before `INSERT`/`UPDATE`/`DELETE`, not before DDL) and `isolation_level=None` where you issue `BEGIN`/`COMMIT` yourself. This lesson uses the legacy default with `with conn:` for DML and explicit `BEGIN` where DDL must be transactional.

## 3. Guided Examples

### 3.1 DB-API basics and parameter binding
- **Goal:** connect, insert, query, map rows, and see why placeholders matter.
- **Run:** `python modules/05-database-access/examples/01_dbapi_basics.py`
- **Code:** [examples/01_dbapi_basics.py](examples/01_dbapi_basics.py)
- **Walkthrough:** `?` (positional) and `:name` (named) bind *values*; identifiers cannot be bound (the `SELECT * FROM ?` error), so validate table names against an allow-list. `fetchone()` returns `None` for no row. A `CHECK` constraint turns invalid data into `sqlite3.IntegrityError`.

```python
hostile = "x'; DROP TABLE tasks; --"
conn.execute("INSERT INTO tasks (title) VALUES (?)", (hostile,))  # stored as plain text
```

- **Expected output:**

  ```
  new id: 1
  1 Write docs todo 2030-01-31
  2 Review PR in_progress None
  3 Deploy done None
  1
  {'id': 1, 'title': 'Write docs', 'status': 'todo', 'due_date': '2030-01-31'}
  None
  stored literally: x'; DROP TABLE tasks; --
  table still there: 4
  OperationalError: near "?": syntax error
  IntegrityError: CHECK constraint failed: status IN ('todo', 'in_progress', 'done')
  ```

### 3.2 Transactions
- **Goal:** atomic multi-statement writes, and what other connections can see.
- **Run:** `python modules/05-database-access/examples/02_transactions.py`
- **Code:** [examples/02_transactions.py](examples/02_transactions.py)
- **Walkthrough:** `with conn:` commits if the block finishes and rolls back if it raises, but it does **not** close the connection (use `contextlib.closing`). The duplicate tag aborts the second insert and the task insert is undone with it. A second connection does not see uncommitted rows. For a connection opened with `isolation_level=None`, a small `transaction()` context manager issues `BEGIN IMMEDIATE`, then `COMMIT` or `ROLLBACK`.

```python
with conn:
    cur = conn.execute("INSERT INTO tasks (title) VALUES (?)", (title,))
    conn.executemany("INSERT INTO task_tags VALUES (?, ?)", [(cur.lastrowid, t) for t in tags])
```

- **Expected output:**

  ```
  created 1
  IntegrityError: UNIQUE constraint failed: task_tags.task_id, task_tags.tag
  tasks: 1 tags: 2
  in transaction: True | reader sees 1
  after commit: reader sees 2
  RuntimeError: boom | tasks: 2
  tasks: 3
  ```

### 3.3 Migrations and connection pragmas
- **Goal:** a versioned schema applied exactly once, plus `foreign_keys`.
- **Run:** `python modules/05-database-access/examples/03_migrations.py`
- **Code:** [examples/03_migrations.py](examples/03_migrations.py)
- **Walkthrough:** SQLite keeps an integer in `PRAGMA user_version`; each migration runs in its own transaction and bumps the version, so start-up can call `migrate()` every time. `PRAGMA foreign_keys = ON` is **per connection and off by default**: without it the orphan insert would succeed. Real projects often use a tool (Alembic, Flyway); the mechanism is the same.
- **Expected output:**

  ```
  before: 0
  after: 2
  again: 2
  ['id', 'title', 'status', 'due_date']
  IntegrityError: FOREIGN KEY constraint failed
  tags left: 0
  ```

### 3.4 Task Tracker repository
- **Goal:** the repository pattern: callers see `Task` objects and a `Protocol`, not SQL.
- **Run:** `python modules/05-database-access/examples/04_task_repository.py`
- **Code:** [examples/04_task_repository.py](examples/04_task_repository.py)
- **Walkthrough:** `TaskRepository` is a structural `Protocol`, so a fake in-memory class or a PostgreSQL class can replace `SqliteTaskRepository` without inheritance. The class owns the SQL, converts `due_date` text to `date`, loads all tags with one `IN (...)` query (generated placeholders, still bound values) and takes a `clock` so `created_at` is deterministic in tests. Pagination returns the page and the total count. Tests use the real engine on `:memory:`.
- **Expected output:**

  ```
  Task(id=1, title='Write docs', status='todo', due_date=datetime.date(2030, 1, 31), tags=('docs', 'p1'), owner_id=None, created_at=datetime.datetime(2030, 1, 1, 9, 0, tzinfo=datetime.timezone.utc))
  ['Write docs', 'Review PR'] of 3
  ['Deploy'] of 1
  None
  True False
  2 left; tags left: 1
  ```

### 3.5 SQLAlchemy Core and PostgreSQL
- **Goal:** build queries as Python expressions; see the PostgreSQL swap without needing a server.
- **Run:** `python modules/05-database-access/examples/05_sqlalchemy_core_and_postgres.py` (optionally with `DATABASE_URL=postgresql://user:pw@localhost/tasks` set)
- **Code:** [examples/05_sqlalchemy_core_and_postgres.py](examples/05_sqlalchemy_core_and_postgres.py)
- **Walkthrough:** `engine.begin()` is a transaction scope, `engine.connect()` a read scope; both return the connection to the pool. Conditions on `tasks.c.status` compile to bound parameters, and the same statement compiles to PostgreSQL syntax by switching the engine URL (`postgresql+psycopg://...`). Note `executemany` takes its column list from the **first** dict, so give every dict the same keys. The `psycopg` part is only reached if `DATABASE_URL` is set.
- **Expected output:**

  ```
  SELECT tasks.id, tasks.title FROM tasks WHERE tasks.status != :status_1 AND tasks.owner_id = :owner_id_1
  1 Write docs
  2 Review PR
  [('done', 1), ('in_progress', 1), ('todo', 1)]
  [] | tables: ['tasks']
  SELECT tasks.id, tasks.title FROM tasks WHERE tasks.status != %(status_1)s::VARCHAR AND tasks.owner_id = %(owner_id_1)s::INTEGER
  DATABASE_URL not set: skipping the PostgreSQL section
  ```

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `f"... WHERE title = '{title}'"` | string formatting feels natural in Python | always `execute(sql, params)`; build only constant fragments dynamically |
| Writes vanish after the program ends | legacy `sqlite3` needs `commit()`; `with conn:` commits **but does not close** | `with conn:` for each write unit and `closing(conn)` (or `conn.close()`) at the end |
| `sqlite3.ProgrammingError: SQLite objects created in a thread can only be used in that same thread` | `check_same_thread=True` by default; FastAPI sync handlers run in a thread pool | one connection per thread/request, or a pool; do not "fix" it by sharing one connection |
| Every connection sees an empty database | each `sqlite3.connect(":memory:")` creates a **new** database | use a temp file, or share one connection in tests |
| Foreign keys and cascades silently ignored | `PRAGMA foreign_keys` is off by default and per connection | run it right after `connect` in one `open_db()` function |
| Half-applied DDL migration | in legacy mode `sqlite3` does not open a transaction before DDL, and `executescript()` commits first | `isolation_level=None` + explicit `BEGIN`/`COMMIT`, one migration per transaction |
| `(5,)` instead of `5`, or `row[3]` everywhere | cursors yield tuples | `row_factory = sqlite3.Row` and map to a dataclass in one place |
| `database is locked` | a forgotten open transaction keeps the write lock | short transactions, `timeout=` on `connect`, `BEGIN IMMEDIATE` for writers |
| `execute("... IN (?)", ([1, 2],))` fails | a placeholder binds one value, not a list | generate `?,?,?` from `len(ids)` and pass the values |
| N+1 queries when loading tags per task | looping `get(id)` for every row | one `IN (...)` query or a join, as in example 3.4 |

## 5. Coming From Other Languages
- **Go:** `database/sql` hides pooling inside `sql.DB`; Python's DB-API has no pool in `sqlite3`, SQLAlchemy's `Engine` plays that role. `rows.Scan` becomes `row_factory` plus your mapping function.
- **Java/C#:** DB-API is like JDBC/ADO.NET; `with conn:` resembles a `TransactionScope`. Unlike EF Core/Hibernate, nothing tracks entities: you map explicitly (or choose the SQLAlchemy ORM, out of scope here).
- **JavaScript/TypeScript:** `better-sqlite3` is synchronous like `sqlite3`; `pg` and `psycopg` use `$1`/`%s` placeholders instead of `?`.
- **Ruby:** ActiveRecord migrations map to Alembic; here `PRAGMA user_version` shows the minimum mechanism.

## 6. Exercises
Each exercise has starter code and tests in [exercises/](exercises/). Solutions are in [solutions/](solutions/). Run all commands from `languages/python/`.

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Task queries | Easy | bound parameters, `LIKE ... ESCAPE`, row mapping, commit | `pytest modules/05-database-access/exercises/ex1_task_queries` |
| 2 | Atomic tags | Medium | transactions, rollback, `with conn:` | `pytest modules/05-database-access/exercises/ex2_atomic_tags` |
| 3 | Migration runner | Stretch | explicit `BEGIN`, `user_version`, failure handling | `pytest modules/05-database-access/exercises/ex3_migrations` |

### Exercise 1: Task queries
- **Task:** implement `add_task`, `get_task` and `search_tasks` in [m05_task_queries.py](exercises/ex1_task_queries/m05_task_queries.py).
- **Acceptance:** [test_m05_task_queries.py](exercises/ex1_task_queries/test_m05_task_queries.py): round trips, `NULL` dates, another connection sees the committed row, hostile search terms stay data, `%` and `_` match literally.
- **Hint:** escape the wildcard characters in the term yourself, then declare the escape character in SQL: `LIKE ? ESCAPE '\'`.

### Exercise 2: Atomic tags
- **Task:** implement `create_task`, `set_tags` and `get_tags` in [m05_atomic_tags.py](exercises/ex2_atomic_tags/m05_atomic_tags.py).
- **Acceptance:** [test_m05_atomic_tags.py](exercises/ex2_atomic_tags/test_m05_atomic_tags.py): a duplicate tag leaves no task row, a failed `set_tags` keeps the old tags, the connection stays usable.
- **Hint:** put every statement of one operation inside a single `with conn:` block.

### Exercise 3: Migration runner
- **Task:** implement `migrate` in [m05_migrations.py](exercises/ex3_migrations/m05_migrations.py).
- **Acceptance:** [test_m05_migrations.py](exercises/ex3_migrations/test_m05_migrations.py): ordered application, idempotence, full rollback of a failing migration (DDL included), refusing newer databases and bad version lists.
- **Hint:** validate the list first, then `BEGIN`, statements, `PRAGMA user_version = N`, `COMMIT`; on any exception `ROLLBACK` and re-raise.

## 7. Recap
- Bind parameters, never format SQL; identifiers come from allow-lists.
- A transaction is a unit of work: `with conn:` (or `engine.begin()`) commits or rolls back for you, `closing()` closes.
- Set per-connection pragmas in one place; keep connections out of threads they were not made in.
- Put SQL behind a repository `Protocol` and test it on a real in-memory database.
- Use plain `sqlite3`/`psycopg` for simple SQL, SQLAlchemy Core when queries are composed dynamically, an ORM only when the team wants entity tracking.

## 8. Self-check Questions
1. Why does `with sqlite3.connect(path) as conn:` still leave a file handle open, and what do you use instead?
2. A migration creates a table and then fails on its second statement. What state is the database in under the legacy transaction mode, and how do you make it all-or-nothing?
3. Why can't a placeholder be used for a table name, and how do you handle a user-selectable sort column safely?
4. Two requests share one module-level `sqlite3` connection in a threaded server. What can go wrong?

## 9. Further Reading
- [sqlite3 module (transaction control, row factories)](https://docs.python.org/3/library/sqlite3.html)
- [PEP 249: Python Database API Specification v2.0](https://peps.python.org/pep-0249/)
- [SQLAlchemy 2.x unified tutorial (Core part)](https://docs.sqlalchemy.org/en/21/tutorial/index.html)
- [psycopg 3 documentation](https://www.psycopg.org/psycopg3/docs/)

---
**Next:** [06 Concurrency](../06-concurrency/README.md)
