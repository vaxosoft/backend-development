# 05. Database Access in TypeScript

> Estimated time: 2h | Difficulty: Intermediate | Prerequisites: 01 (language fundamentals), 04 (serialization)

## Learning Outcomes

After this module you can:

- [ ] Open a database, run queries with prepared statements and bound parameters, and close it cleanly
- [ ] Map rows to typed objects, handling `NULL` and the gap between SQL names and wire names
- [ ] Group writes in a transaction, understand rollback and nesting, and know the isolation guarantees of SQLite
- [ ] Evolve a schema with ordered, versioned migrations
- [ ] Hide SQL behind a repository interface and test it against a real (in-memory) database

## 1. Concept Overview

A service talks to a database through a **driver**: it opens connections (usually pooled), sends SQL text plus parameter values, and receives rows. Three rules cover most of the safety and correctness issues: **bind values, never concatenate them** (SQL injection), **group related writes in a transaction** (all-or-nothing), and **change the schema through versioned migrations**, not by hand.

A **repository** is a small interface (`create`, `get`, `list`, `update`, `delete`) that keeps SQL in one place, so handlers depend on behaviour and tests can use a fake or an in-memory database. Row-to-object mapping is the other recurring job: `NULL` becomes `null`, snake_case columns become the wire names of the domain, relations (task tags) are loaded in batches instead of one query per row.

## 2. How TypeScript Approaches It

- **`better-sqlite3`** (examples and exercises): a _synchronous_ driver. `prepare()` returns a `Statement` with `run`, `get`, `all`, `iterate`; `db.transaction(fn)` wraps a function in `BEGIN`/`COMMIT`/`ROLLBACK`. No server is needed and `:memory:` gives each test a fresh database.
- **Synchronous means blocking.** It is fast for small queries (no network hop), but a slow query freezes the whole event loop and every other request. Keep queries short and indexed; move heavy work to a worker thread (module 06) or use a server database.
- **`pg`** (PostgreSQL): asynchronous, pooled (`pg.Pool`), `$1` placeholders. Shown in one example that only runs when `DATABASE_URL` is set. The repository interface is the seam: swapping the driver changes the implementation, not the callers.
- **No ORM** (see [PROFILE.md](../../PROFILE.md)): hand-written SQL with typed row interfaces keeps the mapping visible. Type arguments such as `db.prepare<[string], UserRow>(...)` are only a promise to the compiler; the driver does not check them.
- **Isolation:** SQLite runs transactions as _serializable_ with a single writer. On PostgreSQL the default is `READ COMMITTED`; choose another level with `BEGIN ISOLATION LEVEL REPEATABLE READ`. A Redis/key-value store is optional in the topic list and not covered here (no extra dependency).

## 3. Guided Examples

### 3.1 Queries and bound parameters

- **Goal:** prepared statements, `run`/`get`/`all`/`iterate`, `NULL` handling and injection safety.
- **Run:** `node modules/05-database-access/examples/01_queries.ts`
- **Code:** [01_queries.ts](examples/01_queries.ts)

  ```ts
  const insert = db.prepare('INSERT INTO users (name, email, bio) VALUES (@name, @email, @bio)');
  insert.run({ name: 'Ada', email: 'ada@example.com', bio: null });
  const byEmail = db.prepare<[string], UserRow>('SELECT * FROM users WHERE email = ?');
  byEmail.get('nobody@example.com'); // undefined when there is no row
  ```

- **Walkthrough:** `lastInsertRowid` and `changes` come back from `run`. `get` returns `undefined` for no row; SQL `NULL` arrives as `null`. The concatenated query with `x' OR '1'='1` matches every row, the bound one matches none. A unique violation throws `Database.SqliteError` with a stable `code`.
- **Expected output:**

  ```
  inserted ids: 1 2 changes: 1
  { id: 1, name: 'Ada', email: 'ada@example.com', bio: null }
  undefined
  [ 'Ada', 'Bob' ]
  name: Bob
  name: Ada
  concatenated: { n: 2 } bound: { n: 0 }
  SQLITE_CONSTRAINT_UNIQUE
  open after close: false
  ```

### 3.2 Transactions

- **Goal:** all-or-nothing writes, savepoints, and the async trap.
- **Run:** `node modules/05-database-access/examples/02_transactions.ts`
- **Code:** [02_transactions.ts](examples/02_transactions.ts)

  ```ts
  const transfer = db.transaction((from: string, to: string, amount: number) => {
    debit.run(amount, from);
    credit.run(amount, to);
  });
  ```

- **Walkthrough:** a `CHECK (balance >= 0)` failure inside the wrapper rolls everything back and rethrows. The same two statements without a transaction leave money missing. A transaction function called inside another becomes a `SAVEPOINT`. An `async` callback is rejected, because the commit would happen before the `await` finishes. `.immediate()` takes the write lock up front.
- **Expected output:**

  ```
  moved 30 [ { owner: 'ada', balance: 70 }, { owner: 'bob', balance: 50 } ]
  rolled back: SQLITE_CONSTRAINT_CHECK
  [ { owner: 'ada', balance: 70 }, { owner: 'bob', balance: 50 } ]
  crash between the two statements
  money vanished: [ { owner: 'ada', balance: 60 }, { owner: 'bob', balance: 50 } ]
  inner failed, outer continues
  [ { owner: 'ada', balance: 61 }, { owner: 'bob', balance: 51 } ]
  TypeError: Transaction function cannot return a promise
  [ { owner: 'ada', balance: 61 }, { owner: 'bob', balance: 56 } ] inTransaction: false
  ```

### 3.3 Migrations

- **Goal:** versioned schema changes with `PRAGMA user_version`.
- **Run:** `node modules/05-database-access/examples/03_migrations.ts`
- **Code:** [03_migrations.ts](examples/03_migrations.ts)
- **Walkthrough:** `migrate` applies every migration newer than the stored version, each in its own transaction together with the version bump, so a failure leaves the database at the last good version. Running it twice is a no-op. The last lines show that `:memory:` is private to one connection.
- **Expected output:**

  ```
  fresh database at version 0
  applied: [ 1, 2, 3 ]
  applied again: [] (idempotent)
  [ 'task_tags', 'tasks' ]
  migration 4 failed: table ok already exists
  version 3 has table "ok": false
  second connection version: 0 tables: 0
  ```

### 3.4 PostgreSQL with `pg` (optional)

- **Goal:** the same ideas with a pooled, asynchronous driver. Compile-checked always, executed only if `DATABASE_URL` is set.
- **Run:** `node modules/05-database-access/examples/04_postgres_optional.ts`
- **Code:** [04_postgres_optional.ts](examples/04_postgres_optional.ts)
- **Walkthrough:** `pool.query(text, values)` uses `$1` placeholders. A transaction needs one dedicated connection (`pool.connect()`), and the client must be `release()`d in a `finally`. `count(*)` arrives as a string (`bigint`).
- **Expected output** (without `DATABASE_URL`):

  ```
  DATABASE_URL is not set: skipping the PostgreSQL example
  ```

### 3.5 A task repository

- **Goal:** the repository pattern in the Task Tracker domain: filters, whitelisted sorting, pagination clamp, tags loaded in one batch, PATCH semantics.
- **Run:** `node modules/05-database-access/examples/05_task_repository.ts`
- **Code:** [05_task_repository.ts](examples/05_task_repository.ts)

  ```ts
  interface TaskRepository {
    create(input: NewTask): Task;
    get(id: number): Task | null;
    list(query: ListQuery): Page<Task>;
    update(id: number, patch: TaskPatch): Task | null;
    delete(id: number): boolean;
  }
  ```

- **Walkthrough:** `list` builds the `WHERE` clause from fixed fragments and bound values; the `ORDER BY` text comes from a `SORTS` whitelist, never from the request. `limit` is clamped to 1-100. Tags are loaded with one `IN (?, ?, ...)` query for the whole page (no N+1). `update` merges the patch inside a transaction (`due_date: null` clears the date). `created_at` comes from an injected clock, so the output is deterministic.
- **Expected output:**

  ```
  {
    id: 1,
    title: 'Write docs',
    status: 'todo',
    due_date: '2030-02-01',
    owner_id: null,
    created_at: '2030-01-01T09:00:00Z',
    tags: [ 'docs', 'p1' ]
  }
  null
  2 [ 'Fix bug' ]
  [ 'Ship:2030-01-15', 'Write docs:2030-02-01', 'Fix bug:null' ]
  100
  {
    id: 1,
    title: 'Write docs',
    status: 'done',
    due_date: null,
    owner_id: null,
    created_at: '2030-01-01T09:00:00Z',
    tags: [ 'docs' ]
  }
  null
  true false
  2
  ```

## 4. Common Pitfalls

| Mistake                                                                        | Why it happens                                                          | Fix                                                                                                       |
| ------------------------------------------------------------------------------ | ----------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------- |
| ``db.prepare(`... WHERE email = '${email}'`)``                                 | template literals look harmless                                         | bind with `?` / `@name`; only whitelisted fragments (sort columns) may be interpolated                    |
| A slow query freezes every request                                             | `better-sqlite3` is synchronous and runs on the event-loop thread       | keep queries short and indexed, paginate, or use a worker thread / server database                        |
| `db.transaction(async () => ...)`                                              | the wrapper commits when the function returns the promise               | keep the body synchronous (the driver throws a `TypeError`); with `pg` use `BEGIN`/`COMMIT` on one client |
| Each test or request opens `new Database(':memory:')` and sees an empty schema | `:memory:` is private to one connection                                 | share one connection object, or use a file path for databases that several connections must see           |
| Foreign keys are not enforced                                                  | SQLite needs `PRAGMA foreign_keys = ON` on every connection             | set it right after opening the connection                                                                 |
| `INSERT OR IGNORE` hides real errors                                           | it also swallows `CHECK` and `NOT NULL` violations, not only duplicates | catch the specific `SqliteError.code` or use `INSERT ... WHERE NOT EXISTS`                                |
| `lastInsertRowid` is typed `number \| bigint`                                  | very large ids may be `bigint`                                          | `Number(...)` when ids stay below 2^53, or `db.defaultSafeIntegers()` and handle `bigint` everywhere      |
| Forgetting `db.close()` / `client.release()`                                   | nothing fails immediately                                               | close in `finally`/`after`; a leaked pg client eventually hangs the pool                                  |
| Trusting `prepare<[], Row>()`                                                  | the type argument is not verified against the actual columns            | map explicitly or validate rows with zod where the schema can drift                                       |

## 5. Coming From Other Languages

- **Java / C#:** `better-sqlite3` is closest to raw JDBC / ADO.NET, minus the connection pool; `pg.Pool` is the pooled counterpart. There is no unit of work or change tracker: you write the SQL.
- **Python:** `db.prepare(...).all()` is like `cursor.execute(...).fetchall()` with `sqlite3`; the difference is that async drivers (`pg`) return promises, as with `asyncpg`.
- **Go:** `database/sql` pools connections and uses `rows.Scan`; here rows come back as plain objects and a transaction is a wrapped function instead of `tx.Commit()`/`defer tx.Rollback()`.
- **JavaScript:** same drivers; TypeScript adds only the row interfaces, which are not checked at runtime.

## 6. Exercises

Starter code and tests are in [exercises/](exercises/); reference solutions in [solutions/](solutions/).

| #   | Title                                                                       | Difficulty | Skill practiced                                         | Run tests                                                                       |
| --- | --------------------------------------------------------------------------- | ---------- | ------------------------------------------------------- | ------------------------------------------------------------------------------- |
| 1   | [User store](exercises/ex1_user_store/user-store.ts)                        | Easy       | prepared statements, `NULL`, error mapping, safe `LIKE` | `node --test "modules/05-database-access/exercises/ex1_user_store/*.test.ts"`   |
| 2   | [Tasks with tags in a transaction](exercises/ex2_task_tags_tx/task-tags.ts) | Medium     | atomic multi-statement writes, rollback, nesting        | `node --test "modules/05-database-access/exercises/ex2_task_tags_tx/*.test.ts"` |
| 3   | [Migration runner](exercises/ex3_migrations/migrations.ts)                  | Stretch    | versioned up/down migrations, per-step transactions     | `node --test "modules/05-database-access/exercises/ex3_migrations/*.test.ts"`   |

### Exercise 1: User store

- **Task:** implement `insertUser` (duplicate email becomes `EmailTakenError` with the driver error as `cause`), `findUserByEmail` (`null` when missing) and `searchUsersByName` (case-insensitive substring, `%` and `_` literal).
- **Acceptance:** all tests in `user-store.test.ts` pass, including the injection attempts.
- **Hint:** check `err instanceof Database.SqliteError` and `err.code`; escape `%`, `_` and `\` in the term and add `ESCAPE '\'` to the `LIKE`.

### Exercise 2: Tasks with tags in a transaction

- **Task:** implement `createTaskWithTags` (all or nothing, also inside an outer transaction) and `renameTag` (merge into an existing tag, return the number of tasks changed).
- **Acceptance:** all tests in `task-tags.test.ts` pass; a failed call leaves no rows behind.
- **Hint:** wrap the statements in `db.transaction(...)()`. `INSERT OR IGNORE` would also ignore a `CHECK` violation: insert only where no row exists yet.

### Exercise 3: Migration runner

- **Task:** implement `schemaVersion` and `migrateTo(db, migrations, target?)` going up and down, validating the list first, rolling back a failing step and reporting it as `MigrationError`.
- **Acceptance:** all tests in `migrations.test.ts` pass.
- **Hint:** `db.pragma('user_version', { simple: true })` reads the version; set it inside the same transaction as the step. The version after a `down` step is the previous known version, not `version - 1`.

## 7. Recap

- Prepare once, bind always; interpolate only values you own (whitelists).
- A transaction is a function: throw to roll back, keep it synchronous, nesting becomes savepoints.
- Migrations are ordered, versioned and applied atomically; the schema version lives in the database.
- A repository returns domain types (`null` for not found) and hides SQL, so handlers and tests do not depend on the driver.
- Use `better-sqlite3` for local and embedded data, `pg` when you need a server database; either way, mind the event loop.

## 8. Self-check Questions

1. Why does `better-sqlite3` offer a synchronous API, and what is the cost in a web server?
2. What happens to the first statement if the second one throws inside `db.transaction(...)`, and what happens if there is no transaction?
3. Why can the `ORDER BY` column not be a bound parameter, and how does the repository stay safe anyway?
4. Why does `list` load tags with one `IN (...)` query instead of one query per task?
5. Two connections open `':memory:'`. Do they share data? What would you change for a multi-connection test?

## 9. Further Reading

- [better-sqlite3 API documentation](https://github.com/WiseLibs/better-sqlite3/blob/master/docs/api.md)
- [SQLite: transactions](https://www.sqlite.org/lang_transaction.html) and [isolation](https://www.sqlite.org/isolation.html)
- [node-postgres: pooling and transactions](https://node-postgres.com/features/pooling)

---

**Next:** [06 Concurrency](../06-concurrency/README.md)
