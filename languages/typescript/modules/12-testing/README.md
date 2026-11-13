# 12. Testing in TypeScript

> Estimated time: 2h | Difficulty: Intermediate | Prerequisites: 01, 02, 03, 05 (and 10/11 for the habits of injecting clocks and sinks)

## Learning Outcomes

After this module you can:

- [ ] Write unit and table-driven tests with `node:test` and `node:assert/strict`.
- [ ] Test HTTP handlers in-process with Fastify `inject()`, without opening a port.
- [ ] Isolate code with fakes, `mock.fn` / `mock.method` and fake timers, and keep every test independent.
- [ ] Run one contract suite against an in-memory fake and a real SQLite store.
- [ ] Measure coverage and write a trustworthy micro-benchmark.

## 1. Concept Overview

A fast test suite lets you change code without fear. The pyramid: many **unit tests** (pure logic, milliseconds), fewer **handler/integration tests** (the HTTP layer plus a real or in-memory store), very few end-to-end tests. To keep unit tests fast and deterministic, the code under test receives its collaborators (store, clock, notifier) as parameters, so a test can substitute a **fake** (a simple working implementation, e.g. an in-memory store), a **stub** (canned answers) or a **mock/spy** (records calls to assert on).

A **contract test** is one suite of behaviour that every implementation of an interface must pass; running it against both the fake and the real database proves the fake is a faithful stand-in. **Coverage** shows which code the tests never ran (a hint, not a goal); a **benchmark** compares performance, but only after warm-up and after checking the compared versions give the same answer.

## 2. How TypeScript Approaches It

- **`node:test`** is built in: `test`, `describe`, `before/after` hooks, `t.after()` for cleanup, `mock` (`mock.fn`, `mock.method`, `mock.timers`), `--test` discovery of `*.test.ts`, `--experimental-test-coverage`, `--watch`. No Jest/Vitest/Sinon in this repo.
- **`node:assert/strict`**: `equal`, `deepEqual`, `throws`, `rejects`, `match`. Always the `strict` flavour (`assert.equal` is `===`).
- **Fastify `inject()`** sends a request straight into the router: no socket, no port, fully deterministic.
- **Interfaces + injection** replace module mocking: because TypeScript types are structural, a plain object literal satisfies an interface, so fakes need no framework. Prefer `mock.fn` for "was it called with...", and a hand-written fake for anything with behaviour.
- Types are erased at runtime and `node` does not type-check, so run `npm run typecheck` as part of the test routine; a test that compiles can still assert nothing.
- Every example is its own `*.test.ts` file; run one with `node --test <file>`, all with `node --test "modules/12-testing/**/*.test.ts"`.

## 3. Guided Examples

Run all commands from `languages/typescript/`. The code under test (Task Tracker rules, a memory repo, a service and a small Fastify app) is in [examples/_shared.ts](examples/_shared.ts); the SQLite repo is in [examples/_sqlite-repo.ts](examples/_sqlite-repo.ts). Test timings differ on every run, so the outputs below have the `(0.4ms)` suffixes and the `duration_ms` line removed (and the zero-valued `cancelled/skipped/todo` counters dropped).

### 3.1 Unit and table-driven tests

- **Goal:** one behaviour per test; a table of cases generating one test each.
- **Run:** `node --test modules/12-testing/examples/01_unit_and_table_tests.test.ts`
- **Code:** [examples/01_unit_and_table_tests.test.ts](examples/01_unit_and_table_tests.test.ts)

```ts
const cases: { name: string; input: string[]; want: string[] }[] = [
  { name: 'lowercases and trims', input: ['  Docs ', 'P1'], want: ['docs', 'p1'] },
  // ...
];
for (const { name, input, want } of cases) {
  test(name, () => assert.deepEqual(normalizeTags(input), want));
}
assert.throws(() => normalizeTags(['x'.repeat(31)]), (err) => err instanceof ValidationError && ...);
```

- **Walkthrough:** `describe` groups, a plain `for` loop over a typed array replaces a "test.each" helper, and a failing row is named in the report. `assert.throws` accepts a predicate, so error _type and fields_ are checked, not just "it threw".
- **Expected output:**

```text
▶ normalizeTags
  ✔ keeps clean tags
  ✔ lowercases and trims
  ✔ drops empty tags
  ✔ removes duplicates, first one wins
  ✔ accepts no tags
  ✔ rejects a tag longer than 30 characters
  ✔ rejects more than 10 distinct tags but not 10 duplicates
  ✔ does not mutate its input
✔ normalizeTags
▶ parseDueDate
  ✔ "2030-01-31" -> 2030-01-31
  ✔ "2028-02-29" -> 2028-02-29
  ✔ "2030-02-29" -> null
  ✔ "2030-13-01" -> null
  ✔ "2030-1-5" -> null
  ✔ "31/01/2030" -> null
  ✔ "" -> null
✔ parseDueDate
ℹ tests 15
ℹ suites 2
ℹ pass 15
ℹ fail 0
```

### 3.2 Fakes, mocks and fake timers

- **Goal:** isolate `TaskService` from its collaborators.
- **Run:** `node --test modules/12-testing/examples/02_fakes_and_mocks.test.ts`
- **Code:** [examples/02_fakes_and_mocks.test.ts](examples/02_fakes_and_mocks.test.ts)

```ts
function setup() {
  // fresh collaborators per test: no shared state
  const repo = createMemoryRepo(() => NOW); // fake
  const notifier = { taskCompleted: mock.fn(async (_t: Task) => {}) }; // mock
  return { repo, notifier, service: new TaskService({ repo, notifier, clock: () => NOW }) };
}
await assert.rejects(service.complete(99), NotFoundError); // always await it
t.mock.timers.enable({ apis: ['setTimeout'] });
t.mock.timers.tick(100);
```

- **Walkthrough:** the repo is a working fake, the notifier a recording mock (`mock.callCount()`, `mock.calls[i].arguments`). The clock is a function returning a fixed `Date`, so "overdue" is deterministic. `t.mock.method` spies on a real method and restores it automatically; `t.mock.timers` fakes `setTimeout`, so a 100 ms debounce is tested in microseconds.
- **Expected output:**

```text
▶ TaskService.complete
  ✔ marks the task done and notifies exactly once
  ✔ completing a done task again does not notify twice
  ✔ unknown ids reject with NotFoundError
  ✔ a failing notifier surfaces its error
✔ TaskService.complete
▶ TaskService.overdue
  ✔ uses the injected clock, ignoring done and undated tasks
✔ TaskService.overdue
▶ spies with t.mock.method
  ✔ wrap a real method and keep its behaviour
✔ spies with t.mock.method
▶ fake timers
  ✔ debounce fires once, after the quiet period, without sleeping
✔ fake timers
ℹ tests 7
ℹ suites 4
ℹ pass 7
ℹ fail 0
```

### 3.3 HTTP handler tests with `inject()`

- **Goal:** test status codes, headers and bodies without a network.
- **Run:** `node --test modules/12-testing/examples/03_http_handler_tests.test.ts`
- **Code:** [examples/03_http_handler_tests.test.ts](examples/03_http_handler_tests.test.ts)

```ts
const res = await app.inject({
  method: 'POST',
  url: '/tasks',
  payload: { title: '  Write docs ' },
});
assert.equal(res.statusCode, 201);
assert.equal(res.headers.location, '/tasks/1');
assert.deepEqual(res.json(), { id: 1, title: 'Write docs', status: 'todo' /* ... */ });
```

- **Walkthrough:** each test builds its own app over a fresh memory repo and registers `t.after(() => app.close())`; an app left open keeps the process alive and the run "hangs". Expected bodies are written out in full, including the shared error envelope, so a wire-format change fails loudly.
- **Expected output:**

```text
▶ POST /tasks
  ✔ creates a task: 201, Location and the full wire format
  ✔ invalid data: 422 with the shared envelope and field messages
  ✔ malformed JSON: 400
✔ POST /tasks
▶ reading and deleting
  ✔ GET unknown id: 404 envelope
  ✔ GET list filters by status and paginates
  ✔ DELETE: 204 once, then 404
✔ reading and deleting
ℹ tests 6
ℹ suites 2
ℹ pass 6
ℹ fail 0
```

### 3.4 Contract tests and an in-memory SQLite integration

- **Goal:** one suite, several implementations.
- **Run:** `node --test modules/12-testing/examples/04_repository_contract.test.ts`
- **Code:** [examples/04_repository_contract.test.ts](examples/04_repository_contract.test.ts)

```ts
function repoContract(name: string, make: () => TaskRepo & { close?: () => void }) {
  /* describe + tests */
}
repoContract('memory repo', () => createMemoryRepo(() => NOW));
repoContract('sqlite repo (:memory:)', () => createSqliteRepo(() => NOW));
```

- **Walkthrough:** the contract lists behaviour (ids, defaults, ordering, pagination clamp, partial update, `null` clears a date, delete result). Every test calls `make()` for a brand-new store (for SQLite a private `:memory:` database: each connection has its own) and closes it in `t.after`. Real SQL needs no container for tests like these; a PostgreSQL repo (module 05) would join the same contract behind `DATABASE_URL`.
- **Expected output:**

```text
▶ memory repo
  ✔ create assigns increasing ids and applies defaults
  ✔ get returns a copy, or undefined for an unknown id
  ✔ list is newest first, filters by status and reports the total
  ✔ list paginates and clamps the limit
  ✔ update changes only the given fields and null clears the date
  ✔ delete reports whether something was removed
✔ memory repo
▶ sqlite repo (:memory:)
  ✔ create assigns increasing ids and applies defaults
  ✔ get returns a copy, or undefined for an unknown id
  ✔ list is newest first, filters by status and reports the total
  ✔ list paginates and clamps the limit
  ✔ update changes only the given fields and null clears the date
  ✔ delete reports whether something was removed
✔ sqlite repo (:memory:)
ℹ tests 12
ℹ suites 2
ℹ pass 12
ℹ fail 0
```

**Coverage** is a flag away. The table (percentages and uncovered line numbers) is deterministic for fixed code:

```bash
node --test --experimental-test-coverage --test-coverage-include="modules/12-testing/examples/_*.ts" "modules/12-testing/examples/0[1-4]*.test.ts"
```

```text
ℹ file               | line % | branch % | funcs % | uncovered lines
ℹ -------------------------------------------------------------------
ℹ    _shared.ts      |  98.82 |    87.36 |  100.00 | 207 217-218
ℹ    _sqlite-repo.ts | 100.00 |    95.45 |   90.00 | 
ℹ all files          |  99.16 |    88.99 |   97.50 | 
```

(Uncovered lines show where a test is missing: line 207 is the 500 branch of the error handler, lines 217-218 the invalid `due_date` check.)

### 3.5 A benchmark you can trust

- **Goal:** compare two ways to compute the distinct tags of many tasks.
- **Run:** `node modules/12-testing/examples/05_benchmark.ts` (add `SHOW_TIMINGS=1` to see the medians)
- **Code:** [examples/05_benchmark.ts](examples/05_benchmark.ts)
- **Walkthrough:** `bench()` warms up so the JIT has compiled the code, times several rounds with `performance.now()` and reports the **median** (outliers from GC or a busy machine do not skew it). It first asserts both versions return the same result: a fast wrong answer is no win. Absolute numbers vary per machine, so only the stable conclusions are printed. Never assert timings in a unit test.
- **Expected output:**

```text
distinct tags: 3000
cases measured: naive (Array.includes) | Set
fastest: Set
Set is at least 5x faster: true
```

## 4. Common Pitfalls

| Mistake                                                                     | Why it happens                                                                           | Fix                                                                                                                                |
| --------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| Forgetting `await` on `assert.rejects` or on an async call before asserting | The test function returns before the promise settles, so it "passes"                     | `await` every promise; make test callbacks `async`; review for floating promises (a stray `service.complete(id);` without `await`) |
| Leaving a Fastify app, server, interval or DB open                          | The test finishes but the event loop is still alive, so `node --test` hangs or times out | `t.after(() => app.close())`, `db.close()`, `timer.unref()`; one app per test                                                      |
| Shared module-level state between tests                                     | A top-level `const repo = ...` is convenient                                             | A `setup()` per test; tests must pass in any order and in isolation (`--test-name-pattern`)                                        |
| Real sleeps (`await sleep(1000)`) and real clocks                           | The easiest way to wait for something                                                    | Inject a clock; use `mock.timers` / a `FakeClock`; await the actual promise or event                                               |
| Using the loose `node:assert` (not `/strict`)                               | The default import is loose: `assert.equal(1, '1')` passes                               | Import `node:assert/strict`                                                                                                        |
| `deepEqual` on objects that contain timestamps or random ids                | Output is non-deterministic                                                              | Inject the clock/id generator, or assert on the stable fields only                                                                 |
| Asserting on mocks too much (call order, exact args of internals)           | Mocks make it easy                                                                       | Prefer state/outcome assertions with fakes; mock only boundaries (notifier, HTTP)                                                  |
| Tests compile-checked never                                                 | `node` strips types without checking them                                                | Run `npm run typecheck` in CI next to the tests                                                                                    |
| `mock.method` left in place                                                 | A mock on a shared object leaks into later tests                                         | Use `t.mock` (restored after the test) or call `mock.restoreAll()` in `afterEach`                                                  |
| Benchmarking cold code, once, with `Date.now()`                             | Quick to write                                                                           | Warm up, repeat, take the median, use `performance.now()`, compare equal outputs                                                   |

## 5. Coming From Other Languages

- **JavaScript:** same `node:test`; the TypeScript gain is that fakes are checked against the interface they replace.
- **Java/C#:** JUnit/xUnit `@Theory`/`[Theory]` are the `for` loop over a table; Mockito/Moq are `mock.fn`, but structural typing makes hand-written fakes a one-liner, so frameworks are used less.
- **Python:** `pytest.mark.parametrize` is the table loop; `unittest.mock.patch` has no direct equivalent: inject dependencies instead of patching modules.
- **Go:** table-driven tests and `httptest` map to the case table and `inject()`; Go's built-in `-race`/`-cover` map to `--experimental-test-coverage` (there is no race detector: JavaScript is single-threaded).

## 6. Exercises

Each exercise has starter code and tests in [exercises/](exercises/); solutions are in [solutions/](solutions/). These are implementation tasks: the tests demonstrate the technique.

| #   | Title                                                                    | Difficulty | Skill practiced                                | Run tests                                                               |
| --- | ------------------------------------------------------------------------ | ---------- | ---------------------------------------------- | ----------------------------------------------------------------------- |
| 1   | [A fake clock](exercises/ex1_fake_clock/fake-clock.ts)                   | Easy       | building a deterministic test double for time  | `node --test "modules/12-testing/exercises/ex1_fake_clock/*.test.ts"`   |
| 2   | [A contract-tested user store](exercises/ex2_memory_store/user-store.ts) | Medium     | in-memory fake that satisfies a contract       | `node --test "modules/12-testing/exercises/ex2_memory_store/*.test.ts"` |
| 3   | [Clock-driven reminders](exercises/ex3_reminders/reminders.ts)           | Stretch    | testing a background loop with fakes and spies | `node --test "modules/12-testing/exercises/ex3_reminders/*.test.ts"`    |

**Exercise 1 - `FakeClock`**

- **Task:** implement `now`, `setTimeout`, `clearTimeout`, `advance` and `pending` so timers fire in order as time is advanced, never in real time.
- **Acceptance:** all tests in `fake-clock.test.ts` pass, including the debouncer that runs on your clock.
- <details><summary>Hint</summary>Keep a list of `{ handle, dueAt, callback }`; in `advance`, repeatedly take the earliest timer due within the window, set `now` to its `dueAt`, run it, and finally set `now` to the target.</details>

**Exercise 2 - `createMemoryUserStore`**

- **Task:** implement the `UserStore` described in the file (validation, case-insensitive unique emails, copies, ids never reused).
- **Acceptance:** every check of the contract in `user-store.test.ts` passes; the same file also shows the contract failing against three deliberately broken stores.
- <details><summary>Hint</summary>Return `{ ...user }` copies from every method, keep a separate `nextId` counter, and compare emails lower-cased.</details>

**Exercise 3 - `startReminders`**

- **Task:** a loop driven by the injected `Clock` that notifies tasks nearing their due time once, retries failures, never double-notifies an in-flight task and can be stopped.
- **Acceptance:** all tests in `reminders.test.ts` pass (they use the provided `FakeClock`, `mock.fn` spies and no real waiting).
- <details><summary>Hint</summary>A `Set` of task ids that are "in flight or done": add before calling `notify`, remove in the `.catch` that calls `onError`.</details>

## 7. Recap

- Test behaviour through public interfaces; one fresh set of collaborators per test.
- Inject clocks, stores and notifiers: fakes for behaviour, `mock.fn` for "was it called".
- `inject()` tests HTTP in-process; always close what you open.
- A contract suite keeps fakes honest against real databases.
- Use coverage to find untested branches and warmed-up medians for benchmarks; assert behaviour, never timings.

## 8. Self-check Questions

1. Why does an un-awaited `assert.rejects(...)` make a test pass even when the code is broken?
2. When would you write a fake instead of a `mock.fn`, and what does a contract test add?
3. A test passes alone but fails after another test: what is the likely cause and the fix?
4. Why is `inject()` preferable to listening on a port in unit tests, and when would you still use a real listener?
5. Why does the benchmark check that both versions return equal results and use the median?

## 9. Further Reading

- Node.js test runner documentation (`node:test`, mocking, coverage)
- Fastify "Testing" guide (`node_modules/fastify/docs/Guides/Testing.md` in this repo)
- Martin Fowler: "Mocks Aren't Stubs"; Kent C. Dodds: "Write tests. Not too many. Mostly integration."

---

**Next:** [13 Time and Standard Library Utilities](../13-time-and-stdlib-utilities/README.md)
