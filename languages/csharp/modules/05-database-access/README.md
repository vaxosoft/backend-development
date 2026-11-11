# 05. Database Access in C#

> Estimated time: 2h | Difficulty: Intermediate | Prerequisites: [01 Language Fundamentals](../01-language-fundamentals/README.md), [04 Serialization](../04-serialization/README.md)

All commands run from `languages/csharp/`. Setup once per shell: `export DOTNET_NOLOGO=1 DOTNET_CLI_TELEMETRY_OPTOUT=1`.

## Learning Outcomes
After this module you can:
- [ ] Open connections, run parameterized commands and read rows with ADO.NET (`Microsoft.Data.Sqlite`), disposing everything.
- [ ] Map rows to types and handle `NULL` in both directions.
- [ ] Use transactions to make multi-statement changes atomic, and know what isolation levels are for.
- [ ] Apply versioned migrations and put queries behind a repository interface.
- [ ] Use EF Core's `DbContext` (change tracking, `AsNoTracking`, `Include`) and swap in PostgreSQL via `Npgsql`.

## 1. Concept Overview
A database driver gives you a *connection* (usually pooled), *commands* (SQL text plus *parameters*) and *readers* (a forward-only cursor over rows). Values are always sent as parameters, never concatenated into SQL: the database parses the SQL once and treats the values as data, which closes the door on SQL injection. Related writes belong in a *transaction* so they commit or roll back together; the *isolation level* decides what concurrent transactions may observe. Schema changes are *migrations*: ordered, versioned, applied once. A *repository* hides all of this behind an interface the rest of the code can fake.

```
caller -> ITaskRepository -> connection (pool) -> command + @parameters -> reader -> record
                                   \-> transaction (commit | rollback)
```

## 2. How C# Approaches It
- **ADO.NET** (`System.Data.Common`): `DbConnection`, `DbCommand`, `DbDataReader`, `DbTransaction`. Providers implement them: `SqliteConnection` (`Microsoft.Data.Sqlite`), `NpgsqlConnection` (`Npgsql`). Code written against the provider types is almost identical across databases.
- **Disposal is the contract:** `await using var conn = ...` returns the connection to the pool; commands and readers are `IAsyncDisposable` too. An uncommitted `DbTransaction` rolls back on dispose.
- **Parameters** are `@name` in the SQL and `command.Parameters.AddWithValue`. A C# `null` is *not* a database NULL: use `DBNull.Value`.
- **EF Core 10** (`Microsoft.EntityFrameworkCore.Sqlite`): `DbContext` = unit of work with change tracking; LINQ is translated to SQL. Most teams use it; ADO.NET (or a micro-ORM) shows what happens underneath.
- **Style to avoid:** string-built SQL, sharing one connection across threads, a singleton `DbContext`, `.Result` on database calls.
- **Databases here:** SQLite so everything runs offline. A private `Data Source=:memory:` database lives as long as its one open connection; several connections to one in-memory database need a *shared-cache* named database (`Mode=Memory;Cache=Shared`) and a "keeper" connection.

## 3. Guided Examples
Run: `dotnet run --project modules/05-database-access/examples -- <name>`.

### 3.1 ADO.NET basics
- **Goal:** connection, command, parameters, reader; SQL injection side by side.
- **Run:** `dotnet run --project modules/05-database-access/examples -- adonet-basics`
- **Code:** [examples/01_AdoNetBasics.cs](examples/01_AdoNetBasics.cs)
  ```csharp
  insert.CommandText = "INSERT INTO users (name, email) VALUES (@name, @email) RETURNING id";
  insert.Parameters.AddWithValue("@name", name);
  var id = (long)(await insert.ExecuteScalarAsync())!;
  ```
- **Walkthrough:** `ExecuteNonQueryAsync` for statements, `ExecuteScalarAsync` for one value (SQLite integers arrive as `long`), `ExecuteReaderAsync` for rows. The hostile name `Bobby'; DROP TABLE users;--` is stored harmlessly; the concatenated query matches every row, the parameterized one none.
- **Expected output:**
  ```
  inserted id=1
  inserted id=2
  inserted id=3
  1 | Ada | ada@example.com
  2 | Grace | grace@example.com
  3 | Bobby'; DROP TABLE users;-- | bobby@example.com
  concatenated: 3 rows match (injection!)
  parameterized: 0 rows match
  ```

### 3.2 NULLs and mapping rows
- **Goal:** nullable columns in and out; a `Map(reader)` function.
- **Run:** `dotnet run --project modules/05-database-access/examples -- nulls-and-mapping`
- **Code:** [examples/02_NullsAndMapping.cs](examples/02_NullsAndMapping.cs)
  ```csharp
  r.IsDBNull(r.GetOrdinal("owner_id")) ? null : r.GetInt32(r.GetOrdinal("owner_id"))
  cmd.Parameters.AddWithValue("@owner", owner ?? (object)DBNull.Value);
  ```
- **Walkthrough:** passing a plain `null` parameter fails at execution; reading a NULL with a typed getter throws, so test `IsDBNull` first. SQLite has no date type: `DateOnly` is stored as `TEXT` `yyyy-MM-dd` and parsed back with an explicit format and `InvariantCulture`.
- **Expected output:**
  ```
  null parameter: InvalidOperationException: Value must be set.
  GetInt32 on NULL: The data is NULL at ordinal 3
  1 With everything due=2030-01-31 owner=7
  2 With nothing due=null owner=null
  ```

### 3.3 Transactions
- **Goal:** two inserts that must succeed together; rollback; isolation level.
- **Run:** `dotnet run --project modules/05-database-access/examples -- transactions`
- **Code:** [examples/03_Transactions.cs](examples/03_Transactions.cs)
  ```csharp
  await using var tx = (SqliteTransaction)await conn.BeginTransactionAsync();
  insert.Transaction = tx;            // every command must be enlisted
  ...
  await tx.CommitAsync();
  ```
- **Walkthrough:** a duplicate tag violates the primary key; the `catch` rolls back, so the task row inserted a moment earlier disappears too. The second half deletes inside a transaction, sees the change, rolls back, and the rows return. SQLite supports `Serializable` (default) and `ReadUncommitted`; servers like PostgreSQL add `ReadCommitted` (their default) and `RepeatableRead`. Higher isolation means fewer anomalies and more conflicts/retries.
- **Expected output:**
  ```
  commit : committed task 1
  failure: rolled back (SQLite error 19: constraint failed)
  tasks=1 tags=2
  inside tx : tags=0
  after undo: tags=2
  ```

### 3.4 EF Core
- **Goal:** the ORM view: entities, `DbContext`, change tracking.
- **Run:** `dotnet run --project modules/05-database-access/examples -- efcore`
- **Code:** [examples/04_EfCoreBasics.cs](examples/04_EfCoreBasics.cs)
  ```csharp
  var readOnly = await db.Tasks.AsNoTracking().ToListAsync();
  var query = db.Tasks.AsNoTracking().Include(t => t.Tags).Where(t => t.DueDate != null).OrderBy(t => t.Title);
  ```
- **Walkthrough:** a `DbContext` is a short-lived unit of work (one per request; in ASP.NET Core `AddDbContext` registers it *scoped*). Editing a tracked entity flips its state to `Modified` and `SaveChangesAsync` writes only the changes. `AsNoTracking` skips that bookkeeping for read-only queries. `Include` loads the tags in the same query instead of one query per task (the N+1 problem). Production schemas use EF migrations (`dotnet ef`, not covered here); `EnsureCreated` is for demos and tests.
- **Expected output:**
  ```
  saved 5 rows
  state before save: Modified
  state after save : Unchanged
  tracked entities  : 2 (of 2)
  AsNoTracking      : 0 (of 2)
  Write the docs: docs,p1 (tags loaded by one Include)
  tasks tagged bug (COUNT runs in SQL): 1
  ```

### 3.5 Task Tracker repository, migrations and the PostgreSQL swap
- **Goal:** an `ITaskRepository` over ADO.NET with versioned migrations.
- **Run:** `dotnet run --project modules/05-database-access/examples -- task-repository`
- **Code:** [examples/05_TaskRepository.cs](examples/05_TaskRepository.cs), [examples/PostgresSwap.cs](examples/PostgresSwap.cs)
  ```csharp
  var where = new List<string>();                    // only fixed fragments are appended
  if (query.Status is not null) { where.Add("t.status = @status"); args.Add(new("@status", query.Status)); }
  cmd.CommandText = Select + filter + " ORDER BY t.created_at DESC, t.id DESC LIMIT @limit OFFSET @offset";
  ```
- **Walkthrough:** `TrackerMigrations` keeps an append-only list of schema steps and records how many ran in `PRAGMA user_version`; each step is a transaction, so the second call applies 0. The repository opens one pooled connection per operation, creates a task and its tags in one transaction, loads tags for a whole page with a correlated `json_group_array` subquery (no N+1), and filters with `EXISTS`. Filters and sort direction are chosen from fixed SQL fragments; user values are parameters. Swapping to PostgreSQL changes the provider types (`NpgsqlConnection`), the connection string and a few SQL dialect details; `PostgresSwap` only connects when `DATABASE_URL=postgres://user:pass@host:5432/db` is set and is not part of any test.
- **Expected output** (without `DATABASE_URL`):
  ```
  migrations applied: 2
  migrations applied: 0
  page of 2/3: Release, Fix bug
  tag p1: Release, Write docs
  in_progress: #2 Fix bug due=null tags=[bug] owner=7 created=2030-01-01T09:01:00Z
  deleted 3: True, again: False
  get 3: null
  PostgreSQL: DATABASE_URL not set, skipping
  ```

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `$"... WHERE email = '{email}'"` | interpolated strings look harmless and the analyzer is off by default | `@email` + `Parameters.AddWithValue`; only whitelisted fragments may be concatenated |
| `AddWithValue("@x", null)` throws "Value must be set" | a C# `null` has no database type | `(object?)value ?? DBNull.Value` |
| `GetInt32` on a nullable column throws | typed getters cannot return NULL | `reader.IsDBNull(i)` first, then read |
| Reader/command/connection not disposed; "database is locked" or pool exhaustion | the object lives until GC finalizes it | `await using` every `IAsyncDisposable`; keep connections short-lived |
| A write happens outside the transaction (or the provider throws about a missing transaction) | `Command.Transaction` is explicit in ADO.NET | set `cmd.Transaction = tx` on every command |
| `DbContext` registered as a singleton or kept in a field | it is not thread-safe and its change tracker grows forever | scoped lifetime (`AddDbContext`) or `IDbContextFactory<T>` for background work |
| N+1 queries: a loop that loads tags per task (lazy loading or manual) | each access is a round trip | `Include`, a join, or one `IN`/subquery for the whole page |
| `:memory:` database "empty" on the next connection | each connection gets its own private database | keep one connection open, or use shared-cache `Mode=Memory;Cache=Shared` with a keeper connection |
| Dates compared as text in the wrong format | SQLite stores whatever you give it | always `yyyy-MM-dd` / RFC 3339 UTC; parse with `InvariantCulture` |
| `PRAGMA foreign_keys` has no effect | it is a per-connection setting, off by default in SQLite | execute it right after opening every connection |

## 5. Coming From Other Languages
- **Java/JDBC + Spring:** `DbConnection`/`DbCommand`/`DbDataReader` ≈ `Connection`/`PreparedStatement`/`ResultSet`; EF Core ≈ JPA/Hibernate with LINQ instead of JPQL; `DbContext` ≈ `EntityManager`. Parameters are named (`@x`), not `?`.
- **TypeScript:** `Microsoft.Data.Sqlite` ≈ `better-sqlite3`; EF Core ≈ Prisma/TypeORM; `await using` ≈ `try/finally` with `close()`.
- **Go:** `database/sql` hides the pool the same way; `await using` replaces `defer rows.Close()`; `IsDBNull` ≈ `sql.NullString`.
- **Python:** `sqlite3` cursors ≈ readers; SQLAlchemy ≈ EF Core; `with` ≈ `using`.

## 6. Exercises
Starters and tests live in `exercises/` (project `M05.Exercises`, helper [TestDb.cs](exercises/TestDb.cs) gives each test its own in-memory database); solutions in `solutions/`.

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Parameterized user store | Easy | parameters, `DBNull`, mapping | `dotnet test modules/05-database-access/exercises --filter "FullyQualifiedName~.Ex1_"` |
| 2 | Atomic task writer | Medium | transactions, rollback | `dotnet test modules/05-database-access/exercises --filter "FullyQualifiedName~.Ex2_"` |
| 3 | Migrator | Stretch | versioned migrations, bookkeeping, failure handling | `dotnet test modules/05-database-access/exercises --filter "FullyQualifiedName~.Ex3_"` |

### Exercise 1: Parameterized user store ([Ex1_UserStore.cs](exercises/Ex1_UserStore.cs))
- **Task:** implement `AddAsync`, `FindByEmailAsync`, `SearchByNameAsync` against a `users` table with a nullable `bio`.
- **Acceptance:** `Ex1_UserStoreTests`: ids returned, `NULL` vs `""` bio kept apart, quotes stored literally, hostile emails find nothing, duplicate email raises `SqliteException`, the name search treats `%` as plain text.
- **Hint:** `instr(lower(name), lower(@fragment))` avoids `LIKE` wildcards altogether.

### Exercise 2: Atomic task writer ([Ex2_TaskWriter.cs](exercises/Ex2_TaskWriter.cs))
- **Task:** `CreateWithTagsAsync` and `ReplaceTagsAsync` must be all-or-nothing.
- **Acceptance:** `Ex2_TaskWriterTests`: a duplicate tag leaves no task and no tags; a failed replace keeps the old tags; the connection stays usable after a failure.
- **Hint:** one `SqliteTransaction`, `cmd.Transaction = tx` everywhere, `CommitAsync` last; disposing an uncommitted transaction rolls it back.

### Exercise 3: Migrator ([Ex3_Migrator.cs](exercises/Ex3_Migrator.cs))
- **Task:** apply pending `Migration`s in version order, recording each in `schema_migrations`.
- **Acceptance:** `Ex3_MigratorTests`: order and result list, fresh database is version 0, re-running is a no-op, a failing migration rolls back only itself (even if it half-ran), duplicate versions rejected up front, an out-of-order older migration is an error.
- **Hint:** the migration's SQL and its bookkeeping `INSERT` go in the same transaction; SQLite DDL is transactional.

## 7. Recap
- Parameters for values, always; connections, commands, readers and transactions are disposables.
- `NULL` is `DBNull`/`IsDBNull`, never a C# `null`.
- Group related writes in one transaction; know your database's default isolation level.
- Migrations are ordered, versioned and transactional; the repository interface keeps SQL out of handlers.
- ADO.NET (or Dapper) for explicit SQL, EF Core for productivity: both need the same discipline about lifetimes and N+1.

## 8. Self-check Questions
1. Why is `WHERE name = @name` safe against `x' OR '1'='1` while the interpolated version is not, even if you escape quotes by hand?
2. A request creates a task, then inserts three tags and the third fails. What exactly guarantees the task row is gone, and what would you see if you forgot `cmd.Transaction = tx`?
3. Why must a web app register `DbContext` as scoped, and what breaks if two requests share one?
4. Your tests use `Data Source=:memory:` with a new connection per repository call and find no tables. Why, and what are two fixes?

## 9. Further Reading
- [ADO.NET overview](https://learn.microsoft.com/dotnet/framework/data/adonet/ado-net-overview) and [Microsoft.Data.Sqlite](https://learn.microsoft.com/dotnet/standard/data/sqlite/)
- [EF Core: change tracking, tracking vs no-tracking, loading related data](https://learn.microsoft.com/ef/core/)
- [Npgsql docs](https://www.npgsql.org/doc/index.html)

---
**Next:** [06 Concurrency](../06-concurrency/README.md)
