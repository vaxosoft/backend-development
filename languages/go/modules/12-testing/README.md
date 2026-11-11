# 12. Testing in Go

> Estimated time: 2h | Difficulty: Intermediate | Prerequisites: 01, 02, 03, 05, 06

## Learning Outcomes
After this module you can:
- [ ] Write table-driven unit tests with subtests, helpers (`t.Helper`) and `errors.Is` assertions
- [ ] Test HTTP handlers in memory (`httptest.NewRecorder`) and over loopback (`httptest.NewServer`)
- [ ] Isolate dependencies behind small interfaces and replace them with hand-written fakes, spies and a controllable clock
- [ ] Run one contract test suite against several implementations, including a real SQLite database
- [ ] Measure with benchmarks, `-cover` and `-race`, and keep tests deterministic

## 1. Concept Overview
A test is a program that states what the code must do and fails when it does not. A good suite is **fast** (run on every save), **deterministic** (no wall clock, no randomness, no network beyond loopback), **independent** (any test alone, any order) and **specific** (a failure says what broke). The levers are the same everywhere: small units with explicit inputs, dependencies passed in so a test can substitute them, and a few slower integration tests that prove the real parts fit together.

```
unit (pure functions, fakes)  ->  handler (recorder / test server)  ->  integration (real SQLite)
 many, microseconds               some, milliseconds                      few, tens of ms
```

## 2. How Go Approaches It

| Need | Tool |
|------|------|
| Test, subtest, parallelism | `testing`, `func TestXxx(t *testing.T)`, `t.Run`, `t.Parallel` |
| Fail with context | `t.Errorf` (continue), `t.Fatalf` (stop), `t.Helper()` (blame the caller's line) |
| Setup and teardown | `t.Cleanup`, `t.TempDir`, `t.Setenv`; `TestMain` only for process-wide setup |
| HTTP | `net/http/httptest`: `NewRequest`, `NewRecorder`, `NewServer`, `NewTLSServer` |
| Fakes | hand-written structs behind consumer-defined interfaces; `testing/fstest.MapFS` for file systems |
| Compare | `==`, `slices.Equal`, `maps.Equal`, `reflect.DeepEqual`, `errors.Is`/`As` |
| Measure | `testing.B` with `b.Loop()`, `-benchmem`, `testing.AllocsPerRun`, `-cover`, `-race`, `-short` |

Idiomatic Go has **no assertion library or mocking framework** (PROFILE: no testify/gomock). Tests are ordinary code: an `if got != want { t.Errorf(...) }`, a table of cases and a 15-line fake. Tests live next to the code in `*_test.go`; use the same package for white-box tests or `pkg_test` for black-box ones.

## 3. Guided Examples
Each example is a package with its code and its tests; "run" means `go test`. `-v` lines such as `=== RUN` and all timings (`0.00s`, the final `ok` line) are omitted from the outputs below.

### 3.1 Table-driven tests and helpers
- **Goal:** the standard shape of a Go unit test.
- **Run:** `go test -v -run 'TestValidate|TestTags' ./modules/12-testing/examples/01_table_tests/`
- **Code:** [titles.go](examples/01_table_tests/titles.go), [titles_test.go](examples/01_table_tests/titles_test.go)
  ```go
  for _, tt := range tests {
      t.Run(tt.name, func(t *testing.T) {
          got, err := Validate(tt.in)
          if !errors.Is(err, tt.wantErr) { t.Fatalf("err = %v, want %v", err, tt.wantErr) }
          assertEqual(t, got, tt.want)
      })
  }
  ```
- **Walkthrough:** a slice of anonymous structs is the table; each row becomes a named subtest you can run alone (`-run TestValidate/blank`). `assertEqual[T comparable]` calls `t.Helper()` so failures point at the test line, not the helper. Errors are compared with `errors.Is` against sentinels, never by message text. `TestSlug` also uses `t.Parallel()`, which is safe because Go 1.22 gives every iteration its own `tt`. Slices are compared with `slices.Equal`, and `Tags(nil)` is checked to be empty but non-nil (it must marshal to `[]`).
- **Expected output:**
  ```
  --- PASS: TestValidate
      --- PASS: TestValidate/trims
      --- PASS: TestValidate/empty
      --- PASS: TestValidate/blank
      --- PASS: TestValidate/200_runes_ok
      --- PASS: TestValidate/201_runes_too_long
  --- PASS: TestTags
  PASS
  ```

### 3.2 HTTP handler tests
- **Goal:** test routing, status codes, headers and JSON without a network.
- **Run:** `go test -v ./modules/12-testing/examples/02_handler_tests/`
- **Code:** [taskapi.go](examples/02_handler_tests/taskapi.go), [taskapi_test.go](examples/02_handler_tests/taskapi_test.go)
  ```go
  req := httptest.NewRequest(method, target, strings.NewReader(body))
  rec := httptest.NewRecorder()
  handler.ServeHTTP(rec, req)          // rec.Code, rec.Header(), rec.Body
  ```
- **Walkthrough:** `NewRecorder` calls the handler directly: no port, no goroutine, microseconds. The tiny `do` and generic `decode[T]` helpers keep each test to a few lines. The table covers success, malformed JSON (400), unknown field (400) and validation failures (422 with a `fields` entry), asserting on the error **code**, not the message. The mux's 405 and its `Allow` header are tested too. One last test uses `httptest.NewServer` (a real loopback listener, `t.Cleanup(srv.Close)`) to prove what the recorder skips: the TCP/HTTP client path and the `Location` header being followable.
- **Expected output:**
  ```
  --- PASS: TestCreate
      --- PASS: TestCreate/valid
      --- PASS: TestCreate/malformed_JSON
      --- PASS: TestCreate/unknown_field
      --- PASS: TestCreate/blank_title
      --- PASS: TestCreate/bad_status
  --- PASS: TestGetAndNotFound
  --- PASS: TestListFiltersByStatus
  --- PASS: TestMethodNotAllowed
  --- PASS: TestOverRealServer
  PASS
  ```

### 3.3 Fakes, spies and a controllable clock
- **Goal:** isolate a service from its store, mailer and the clock.
- **Run:** `go test -v ./modules/12-testing/examples/03_fakes_and_clocks/`
- **Code:** [reminders.go](examples/03_fakes_and_clocks/reminders.go), [reminders_test.go](examples/03_fakes_and_clocks/reminders_test.go)
  ```go
  type Notifier interface { Notify(ctx context.Context, owner, message string) error } // declared by the consumer
  var _ Notifier = (*recorder)(nil)                                                       // the fake keeps up with it
  clock.Advance(48 * time.Hour)                                                           // no sleeping
  ```
- **Walkthrough:** `Service` takes three one- or two-method interfaces. The test file defines `fakeStore` (canned data or error), `recorder` (a spy that remembers messages and fails for chosen owners) and `fakeClock`. The tests show behaviour, not calls: who got which message, that one failed notification does not stop the others (`errors.Join`, found with `errors.Is`), that store errors are wrapped, and that moving the clock makes a task overdue. `LoadSeed` takes an `fs.FS`, so `fstest.MapFS` replaces the disk, including the `fs.ErrNotExist` case.
- **Expected output:**
  ```
  --- PASS: TestSendOverdue
  --- PASS: TestOneFailedNotificationDoesNotStopTheRest
  --- PASS: TestStoreFailureIsWrapped
  --- PASS: TestLoadSeed
  PASS
  ```

### 3.4 Benchmarks, allocations, coverage, race detector
- **Goal:** measure instead of guessing; know what each flag reports.
- **Run:** `go test -v ./modules/12-testing/examples/04_benchmarks_and_coverage/` then `go test -run '^$' -bench . -benchmem ./modules/12-testing/examples/04_benchmarks_and_coverage/`, `go test -cover ...` and `go test -race ...` (the last needs cgo/gcc)
- **Code:** [tagstats.go](examples/04_benchmarks_and_coverage/tagstats.go), [tagstats_test.go](examples/04_benchmarks_and_coverage/tagstats_test.go)
  ```go
  for b.Loop() { impl.fn(in) }                         // Go 1.24+; setup before the loop is not timed
  testing.AllocsPerRun(100, func() { DedupSlow(in) })  // exactly 1: deterministic, so it can be asserted
  ```
- **Walkthrough:** benchmarks are named `BenchmarkXxx` and use `b.Run` for sub-cases; `-run '^$'` skips the tests. The output has the shape `BenchmarkDedup/n=10/slice-32  <iterations>  <n> ns/op  <n> B/op  <n> allocs/op`: the numbers differ per machine, so they are not asserted. They do teach something: for 10 tags the "slow" slice scan beats the map, and only at 1000 tags does the map win by a wide margin, so measure before optimising. Allocation counts *are* stable and make good regression tests. `-cover` reports statement coverage (96.0%: `Describe`'s last branch is untested on purpose; `go test -coverprofile=c.out` + `go tool cover -func=c.out` or `-html=c.out` shows where), and `-race` runs `TestCounterIsSafeForConcurrentUse` under the race detector.
- **Expected output** (`go test -v`; `go test -cover` prints `coverage: 96.0% of statements`):
  ```
  --- PASS: TestDedupVariantsAgree
  --- PASS: TestDescribe
  --- PASS: TestDedupAllocations
  --- PASS: TestCounterIsSafeForConcurrentUse
  PASS
  ```

### 3.5 Task Tracker: one contract, two stores
- **Goal:** keep a fast in-memory fake honest by running the same tests against the real SQLite store.
- **Run:** `go test -v ./modules/12-testing/examples/05_store_contract/` (add `-short` to skip the SQLite integration tests)
- **Code:** [store.go](examples/05_store_contract/store.go), [sqlite.go](examples/05_store_contract/sqlite.go), [store_test.go](examples/05_store_contract/store_test.go)
  ```go
  func testStore(t *testing.T, newStore func(t *testing.T) Store) { /* the contract */ }
  func TestMemStore(t *testing.T)    { testStore(t, func(*testing.T) Store { return NewMemStore() }) }
  func TestSQLiteStore(t *testing.T) { /* skip if testing.Short(); temp-file DB per subtest */ }
  ```
- **Walkthrough:** `testStore` is the specification (ids, defaults, not-found, ordering, filtering, defensive copies, empty-not-nil lists). Each implementation must pass it, so a handler test can inject `MemStore` knowing that production behaves the same. SQLite gets a fresh database file in `t.TempDir()` per subtest and is closed with `t.Cleanup`; an in-memory SQLite database would be a different empty database on every pooled connection. A separate test reopens the file to prove persistence. `testing.Short()` is the conventional switch for slow tests.
- **Expected output** (`-short`; a full run adds `PASS` lines for the SQLite subtests, which take a few milliseconds each):
  ```
  --- PASS: TestMemStore
      --- PASS: TestMemStore/create_assigns_increasing_ids_and_defaults
      --- PASS: TestMemStore/get_returns_what_was_stored
      --- PASS: TestMemStore/no_tags_round-trips_as_empty,_not_nil
      --- PASS: TestMemStore/callers_cannot_change_stored_data
      --- PASS: TestMemStore/missing_ids_report_ErrNotFound
      --- PASS: TestMemStore/list_is_ordered_and_filterable
      --- PASS: TestMemStore/delete_removes_only_that_task
  --- SKIP: TestSQLiteStore
  --- SKIP: TestSQLiteStorePersistsAcrossReopen
  PASS
  ```

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| Asserting on error strings | `err.Error() == "..."` is the quickest check | Sentinels with `errors.Is`, typed errors with `errors.As`; assert on HTTP status and error `code`, not message |
| Shared mutable state across `t.Parallel` tests | A package-level map, counter or temp path is touched by several parallel subtests | Build fresh state per test; `t.TempDir()`; run `go test -race` |
| `t.Fatal` from a spawned goroutine | `FailNow` must run on the test goroutine; elsewhere it only exits that goroutine | Send errors back over a channel or use `t.Error` and return |
| `time.Sleep` to wait for something | It is the obvious tool and it makes tests slow and flaky | Inject a clock, wait on a channel or condition with a deadline (exercise 1) |
| Forgetting `t.Helper()` | Helpers are small and "work" without it | Call it first in every helper so failures show the caller's line |
| Closure over the loop variable in subtests (pre-1.22 code) | Parallel subtests all saw the last row | Go 1.22+ gives each iteration its own variable; in older code copy `tt := tt` |
| Using `httptest.NewServer` for everything | It works and feels realistic | Use `NewRecorder` for handler logic, `NewServer` only for client/transport behaviour; always `defer srv.Close()` |
| Mocks that restate the implementation | Verifying every call makes tests break on refactors | Fakes with behaviour (in-memory store, recording spy); assert outcomes |
| In-memory SQLite with a pool | Every connection is its own empty database | Temp file per test, or `SetMaxOpenConns(1)` |
| Trusting benchmark numbers or asserting on them | Timings vary per machine and load | Compare relative results, use `-benchmem`, assert allocations with `testing.AllocsPerRun` |
| Chasing 100% coverage | The number is easy to read | Use `-coverprofile` to find untested *behaviour*; ignore trivial lines |

## 5. Coming From Other Languages
- **Java/C# (JUnit, xUnit, Mockito, Moq):** no attributes, runners, assertion DSL or mock generators. A test is a function, a table is a slice, a mock is a struct you write against an interface that the *consumer* defines. `t.Fatalf` replaces `Assert.*`, and `t.Cleanup` replaces `[TearDown]`.
- **Python/JS (pytest, Jest):** the table plays the role of `parametrize`/`test.each`; there are no fixtures or `monkeypatch`, so dependencies are passed in rather than patched. Output is plain text; run a single case with `-run Name/sub`.
- **Rust:** unit tests live beside the code like `#[cfg(test)]`, but `*_test.go` files are separate and can be black-box (`package x_test`); there are no doc-test equivalents beyond `Example` functions.

## 6. Exercises
Starter code and tests are in [exercises/](exercises/); reference solutions in [_solutions/](_solutions/). Each exercise is an implementation task whose tests demonstrate a technique. Also run them with `-race`.

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Fake clock | Easy | a clock the test controls, deterministic timers | `go test -race ./modules/12-testing/exercises/ex1_fake_clock/` |
| 2 | HTTP check helpers | Medium | test helpers, `t.Helper`, testing a helper with a fake `T` | `go test ./modules/12-testing/exercises/ex2_http_check/` |
| 3 | Store contract and fault injection | Stretch | contract tests, defensive copies, a failing wrapper | `go test -race ./modules/12-testing/exercises/ex3_store_contract/` |

### Exercise 1: Fake clock
- **Task:** implement `Fake` (`Now`, `Since`, `Set`, `Advance`, `After`, `Pending`) so that timers fire exactly when fake time reaches their deadline.
- **Acceptance:** `TestNowSetAndSince`, `TestAfterFiresOnlyWhenTheDeadlineIsReached`, `TestOneBigAdvanceFiresEveryDueTimer`, `TestSetForwardFiresTimersButBackwardDoesNot`, `TestNonPositiveDelayIsImmediate`, `TestAdvanceNegativePanics`, `TestDrivingAGoroutineWithoutSleeping`, `TestConcurrentUse`.
- <details><summary>Hint</summary>Keep a mutex, the current time and a slice of <code>{deadline, chan}</code>. Give each channel capacity 1 so firing never blocks, and fire inside the lock.</details>

### Exercise 2: HTTP check helpers
- **Task:** build the helper package: `Do` (in-memory request, JSON or raw body), `Response.JSON`, `ExpectStatus`, `ExpectHeader`, `ExpectError`. The tests pass a recording fake `T` to check exactly how many errors and fatals each helper reports.
- **Acceptance:** `TestDoEncodesJSONBodies`, `TestDoSendsStringsAndBytesUntouched`, `TestDoWithoutBody`, `TestDoStopsTheTestWhenTheBodyCannotBeEncoded`, `TestJSON`, `TestExpectStatus`, `TestExpectHeader`, `TestExpectError`.
- <details><summary>Hint</summary>Start every helper with <code>t.Helper()</code>. Use <code>t.Errorf</code> for checks (the test can keep going), <code>t.Fatalf</code> when continuing makes no sense, and <code>mime.ParseMediaType</code> for the Content-Type.</details>

### Exercise 3: Store contract and fault injection
- **Task:** implement `MemStore` so it passes the contract in the tests (validation, ids never reused, defensive copies, concurrency), the `Faulty` wrapper that fails after N calls, and `DeleteDone`, whose error path only fault injection can reach.
- **Acceptance:** `TestMemStoreContract`, `TestFaultyIsTransparentUntilItFails`, `TestFaultyFailsAfterOkCalls`, `TestFaultyCountsConcurrentCalls`, `TestDeleteDone`, `TestDeleteDoneStopsAtTheFirstError`, `TestDeleteDoneListFailure`.
- <details><summary>Hint</summary><code>slices.Clone</code> the tags on the way in and out; validate before taking an id. In <code>Faulty</code>, count the call under a mutex first, then either return the injected error or forward to the inner store.</details>

## 7. Recap
- Tests are plain Go: a table, `t.Run`, `t.Errorf`, helpers with `t.Helper()`; compare errors with `errors.Is`.
- Test handlers with `httptest.NewRecorder`; use `NewServer` only when the transport matters.
- Depend on small interfaces, then fake them by hand; inject the clock instead of sleeping.
- Run one contract suite against the fake and the real store; mark slow tests with `testing.Short()`.
- Benchmark before optimising, assert allocations not nanoseconds, and run `go test -race -cover` in CI.

## 8. Self-check Questions
1. Why does a helper that calls `t.Fatalf` but not `t.Helper()` make failures harder to read?
2. When would you choose `httptest.NewServer` over `NewRecorder`, and what does the recorder not exercise?
3. Why is the `Notifier` interface declared in the consumer's package instead of next to the SMTP implementation?
4. What bug does the contract test "callers never share memory with the store" catch, and why would a map-based fake hide it from handler tests?
5. A benchmark shows the slice version faster for 10 items and slower for 1000. What do you do next?

## 9. Further Reading
- [Go `testing` package](https://pkg.go.dev/testing) and [Add a test (tutorial)](https://go.dev/doc/tutorial/add-a-test)
- [Go Wiki: TableDrivenTests](https://go.dev/wiki/TableDrivenTests) and [Go Wiki: TestComments](https://go.dev/wiki/TestComments)
- [`net/http/httptest`](https://pkg.go.dev/net/http/httptest), [`testing/fstest`](https://pkg.go.dev/testing/fstest) and [Go blog: Cover](https://go.dev/blog/cover)

---
**Next:** [13 Time and Standard Library Utilities](../13-time-and-stdlib-utilities/README.md)
