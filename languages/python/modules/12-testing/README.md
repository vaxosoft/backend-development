# 12. Testing in Python

> Estimated time: 2 h | Difficulty: Intermediate | Prerequisites: [02 HTTP server](../02-http-server/README.md), [05 Database access](../05-database-access/README.md), [07 HTTP client](../07-http-client/README.md); [11 Auth](../11-auth-and-security-basics/README.md) helps

## Learning Outcomes

After this module you can:

- [ ] Write unit tests with plain `assert`, `pytest.mark.parametrize` and `pytest.raises`
- [ ] Use fixtures (setup/teardown, scope, factories), `tmp_path`, `monkeypatch`, `capsys` and `caplog`
- [ ] Replace collaborators with fakes, stubs and `unittest.mock` (with `autospec`) and inject clocks
- [ ] Test HTTP handlers with FastAPI's `TestClient`, an async client and `dependency_overrides`
- [ ] Run the same contract tests against several implementations and test against a real SQLite database
- [ ] Measure coverage with `pytest-cov` and micro-benchmarks with `timeit`

## 1. Concept Overview

A test is a small program that runs your code with known inputs and checks the outputs. Good suites have a **pyramid** shape: many fast unit tests (pure logic), fewer integration tests (real database, real HTTP stack in-process), very few end-to-end tests.

```text
            /\        a few: real server + real DB over a socket
           /  \       some:  HTTP handler + real SQLite, no network
          /____\      many:  pure functions and services with fakes
```

What makes tests useful: they are **deterministic** (no wall-clock time, no randomness, no network), **isolated** (no shared state between tests, any order), **fast**, and they check **behaviour** rather than implementation details. The main tool for that is *dependency injection*: pass in the clock, the store or the notifier, so a test can substitute a controllable double.

| Double | What it is | Use it to |
|--------|-----------|-----------|
| Stub | returns canned answers | make a dependency return a fixed value or error |
| Fake | a simple working implementation (in-memory repo, fake clock) | run real logic without the slow/external part |
| Spy | records calls | assert that something was (not) called |
| Mock | spy with expectations (`assert_called_once_with`) | verify an interaction that has no visible result |

## 2. How Python Approaches It

| Need | Tool |
|------|------|
| Runner and assertions | `pytest`: plain `assert` with rewritten failure messages; files `test_*.py`, functions `test_*` |
| Table-driven tests | `@pytest.mark.parametrize` (with `pytest.param(..., id=...)`) |
| Expected exceptions | `with pytest.raises(Error, match="regex")` |
| Setup / teardown | fixtures: `@pytest.fixture`, `yield` for teardown, `scope=`, factory fixtures |
| Built-in fixtures | `tmp_path`, `monkeypatch`, `capsys`, `caplog`, `request` |
| Mocks | `unittest.mock` (`Mock`, `create_autospec`, `patch`) from the stdlib |
| HTTP handlers | `fastapi.testclient.TestClient`; `httpx.AsyncClient(transport=ASGITransport(app))` for async tests |
| Replace wiring | `app.dependency_overrides[get_service] = lambda: fake` |
| Async tests | the `anyio` pytest plugin; this repo sets `anyio_mode = "auto"`, so `async def test_...` just works |
| Coverage / timing | `pytest-cov` (`--cov`), `timeit` (`python -m timeit`) |

Idioms: tests are plain functions (no `TestCase` classes needed); prefer **fakes over mocks** for collaborators you own and `create_autospec` when you do mock; inject the clock instead of patching `datetime`; use `tmp_path` instead of writing to the repo; keep fixtures small and put shared ones in `conftest.py` (not used here, so each example stays self-contained). `unittest` and `doctest` also exist, but pytest is the standard runner.

## 3. Guided Examples

The examples are runnable test files that exercise the small Task Tracker core in [examples/m12_tasks.py](./examples/m12_tasks.py) and the FastAPI app in [examples/m12_app.py](./examples/m12_app.py). Run each with `pytest` (the listings of 3.1, 3.4 and 3.5 use `-vv --no-header`; `python <file>` does the same as `pytest <file>`). Output shown is from a real run; the duration in the last line varies. With the pinned versions, importing `TestClient` prints a harmless `StarletteDeprecationWarning` about `httpx`; the commands for the HTTP examples add `-W ignore` to hide it.

### 3.1 Unit tests and parametrize

- **Goal:** test pure logic with plain `assert`, one case per table row.
- **Run:** `pytest modules/12-testing/examples/01_unit_tests.py -vv --no-header`
- **Code:** [examples/01_unit_tests.py](./examples/01_unit_tests.py)
- **Walkthrough:** each `test_*` function is collected automatically. `parametrize` turns one function into one test per row and `pytest.param(..., id=)` gives each case a readable name, so a failure says `test_normalize_tags[blanks-dropped]`. `pytest.raises(..., match=)` checks the type *and* the message.

```python
@pytest.mark.parametrize(
    ("raw", "expected"),
    [pytest.param([" docs ", "docs", "DOCS"], ["docs"], id="duplicates-after-cleanup"), ...],
)
def test_normalize_tags(raw: list[str], expected: list[str]) -> None:
    assert normalize_tags(raw) == expected
```

- **Expected output:**

  ```text
  ============================= test session starts ==============================
  collecting ... collected 10 items

  modules/12-testing/examples/01_unit_tests.py::test_title_is_trimmed PASSED [ 10%]
  modules/12-testing/examples/01_unit_tests.py::test_invalid_titles_are_rejected[empty] PASSED [ 20%]
  modules/12-testing/examples/01_unit_tests.py::test_invalid_titles_are_rejected[blank] PASSED [ 30%]
  modules/12-testing/examples/01_unit_tests.py::test_invalid_titles_are_rejected[201-chars] PASSED [ 40%]
  modules/12-testing/examples/01_unit_tests.py::test_normalize_tags[empty] PASSED [ 50%]
  modules/12-testing/examples/01_unit_tests.py::test_normalize_tags[lower-cased] PASSED [ 60%]
  modules/12-testing/examples/01_unit_tests.py::test_normalize_tags[duplicates-after-cleanup] PASSED [ 70%]
  modules/12-testing/examples/01_unit_tests.py::test_normalize_tags[blanks-dropped] PASSED [ 80%]
  modules/12-testing/examples/01_unit_tests.py::test_normalize_tags[order-kept] PASSED [ 90%]
  modules/12-testing/examples/01_unit_tests.py::test_more_than_ten_tags_is_an_error PASSED [100%]

  ============================== 10 passed in 0.68s ==============================
  ```

### 3.2 Fixtures, `tmp_path`, `monkeypatch`, `caplog`

- **Goal:** share setup without sharing state.
- **Run:** `pytest modules/12-testing/examples/02_fixtures.py`
- **Code:** [examples/02_fixtures.py](./examples/02_fixtures.py)
- **Walkthrough:** a fixture is injected by *parameter name*. Code after `yield` is teardown and runs even when the test fails. `make_task` is a *factory fixture*: it returns a function, so each test decides how many tasks it needs. The default scope is `function` (fresh per test); `scope="module"` builds once and is only safe for immutable data. `tmp_path` is a unique directory, `monkeypatch.setenv` is undone automatically, `caplog` and `capsys` capture logging and printing.

```python
@pytest.fixture
def repo() -> Iterator[SqliteTaskRepository]:
    repo = SqliteTaskRepository()  # setup
    yield repo
    repo.close()  # teardown
```

- **Expected output:** (default quiet mode: one dot per passing test)

  ```text
  ........                                                                 [100%]
  8 passed in 0.29s
  ```

### 3.3 Fakes, mocks and the injected clock

- **Goal:** isolate the unit under test and make time deterministic.
- **Run:** `pytest modules/12-testing/examples/03_fakes_and_mocks.py`
- **Code:** [examples/03_fakes_and_mocks.py](./examples/03_fakes_and_mocks.py)
- **Walkthrough:** `TaskService` takes a `clock` callable, so the tests pass a `FakeClock` and assert exact timestamps; a hand-written `RecordingNotifier` is a fake that is easy to read. `create_autospec(TaskRepository, instance=True)` builds a mock that has the real signatures: calling `add("title")` with the wrong arguments or a method that does not exist **fails**, while a plain `Mock()` accepts anything and hides bugs. The last test shows the rule for `monkeypatch`/`patch`: replace the name *where it is looked up* (`m12_tasks.datetime`), not where it is defined.

```python
repo = create_autospec(TaskRepository, instance=True)
repo.add.return_value = Task(1, "Write docs", "todo", (), NOON)
TaskService(repo, clock=FakeClock()).create("  Write docs ")
repo.add.assert_called_once_with("Write docs", (), NOON)
```

- **Expected output:**

  ```text
  .......                                                                  [100%]
  7 passed in 0.36s
  ```

### 3.4 HTTP handler tests with `dependency_overrides`

- **Goal:** test routes, status codes and the error envelope without a network.
- **Run:** `pytest modules/12-testing/examples/04_http_tests.py -vv --no-header -W ignore`
- **Code:** [examples/04_http_tests.py](./examples/04_http_tests.py), app in [examples/m12_app.py](./examples/m12_app.py)
- **Walkthrough:** the app gets its `TaskService` through `Depends(get_service)`. The `app` fixture builds a fresh app, overrides that dependency with a service that has an in-memory repo and a frozen clock, and clears the overrides afterwards. `TestClient` calls the ASGI app in-process (no port). The last test uses `httpx.AsyncClient(transport=httpx.ASGITransport(app=app))` from an `async def` test, which the anyio plugin runs on the event loop.

```python
service = TaskService(InMemoryTaskRepository(), clock=lambda: NOON)
app.dependency_overrides[get_service] = lambda: service
response = TestClient(app).post("/tasks", json={"title": "  Write docs ", "tags": ["Docs", "docs"]})
assert response.status_code == 201 and response.headers["Location"] == "/tasks/1"
```

- **Expected output:**

  ```text
  ============================= test session starts ==============================
  collecting ... collected 9 items

  modules/12-testing/examples/04_http_tests.py::test_create_returns_201_location_and_the_task PASSED [ 11%]
  modules/12-testing/examples/04_http_tests.py::test_get_returns_what_was_created PASSED [ 22%]
  modules/12-testing/examples/04_http_tests.py::test_unknown_id_uses_the_error_envelope PASSED [ 33%]
  modules/12-testing/examples/04_http_tests.py::test_invalid_bodies_are_422[blank] PASSED [ 44%]
  modules/12-testing/examples/04_http_tests.py::test_invalid_bodies_are_422[too-long] PASSED [ 55%]
  modules/12-testing/examples/04_http_tests.py::test_invalid_bodies_are_422[wrong-type] PASSED [ 66%]
  modules/12-testing/examples/04_http_tests.py::test_invalid_bodies_are_422[missing] PASSED [ 77%]
  modules/12-testing/examples/04_http_tests.py::test_malformed_json_is_a_400 PASSED [ 88%]
  modules/12-testing/examples/04_http_tests.py::test_async_client_talks_to_the_app_in_process[asyncio] PASSED [100%]

  ============================== 9 passed in 0.57s ===============================
  ```

### 3.5 Contract and integration tests on SQLite

- **Goal:** one test suite, several implementations; then a test through the real database.
- **Run:** `pytest modules/12-testing/examples/05_integration_tests.py -vv --no-header -W ignore -k "ids_are_assigned or http_to"` (drop `-k` to run all 19)
- **Code:** [examples/05_integration_tests.py](./examples/05_integration_tests.py)
- **Walkthrough:** the `repo` fixture is parametrized (`params=[...]`), so **every** test runs against the in-memory repository, SQLite `:memory:` and a SQLite file in `tmp_path`: that is a *contract test*, and it is how you keep a fake honest. `test_http_to_sqlite_file_and_back` goes HTTP -> service -> SQLite file and reads the row back through a second connection. SQLite in memory is the "container-free" integration database; for PostgreSQL you would run the same contract against a real server (e.g. started by CI or `testcontainers`) and skip when `DATABASE_URL` is unset (`pytest.mark.skipif`).
- **Expected output:**

  ```text
  ============================= test session starts ==============================
  collecting ... collected 19 items / 15 deselected / 4 selected

  modules/12-testing/examples/05_integration_tests.py::test_ids_are_assigned_in_order[memory] PASSED [ 25%]
  modules/12-testing/examples/05_integration_tests.py::test_ids_are_assigned_in_order[sqlite-memory] PASSED [ 50%]
  modules/12-testing/examples/05_integration_tests.py::test_ids_are_assigned_in_order[sqlite-file] PASSED [ 75%]
  modules/12-testing/examples/05_integration_tests.py::test_http_to_sqlite_file_and_back PASSED [100%]

  ======================= 4 passed, 15 deselected in 0.51s =======================
  ```

### 3.6 Coverage and micro-benchmarks

- **Goal:** find untested code; time a small function.
- **Run:**

  ```console
  pytest modules/12-testing/examples/0[1-5]_*.py --cov=m12_tasks --cov=m12_app --cov-report=term-missing -W ignore
  python -m timeit -s "import sys; sys.path.insert(0, 'modules/12-testing/examples')" -s "from m12_tasks import normalize_tags" "normalize_tags(['Docs', ' p1 ', 'docs'])"
  ```

- **Walkthrough:** `--cov=<module>` measures the named modules; `term-missing` lists the line numbers that never ran. Here `m12_tasks.py` lines 159-164 are `TaskService.set_status`, which no example tests, and `m12_app.py` line 40 is the production wiring that the tests always override. Coverage shows what was *not* run, not that what ran is correct; 100 % is not a goal. `timeit` repeats a statement and reports the best of several runs: it prints one line such as `2000000 loops, best of 5: 174 nsec per loop` (the number depends on the machine). Benchmark only when you have a performance question, and compare before/after on the same machine.
- **Expected output** (coverage; time varies):

  ```text
  Name                                       Stmts   Miss  Cover   Missing
  ------------------------------------------------------------------------
  modules/12-testing/examples/m12_app.py        45      1    98%   40
  modules/12-testing/examples/m12_tasks.py      96      6    94%   159-164
  ------------------------------------------------------------------------
  TOTAL                                        141      7    95%
  53 passed in 1.37s
  ```

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `patch("datetime.datetime")` or `monkeypatch.setattr(datetime, ...)` has no effect | The code under test did `from datetime import datetime`, so the name it uses lives in *its own* module | Patch where the name is looked up (`monkeypatch.setattr(m12_tasks, "datetime", ...)`), or better inject a clock |
| `Mock()` accepts any call, so a test passes while the code is wrong | Plain mocks have no signature | `create_autospec(Cls, instance=True)`; prefer a hand-written fake for things you own |
| A mutable module- or session-scoped fixture leaks state between tests | Wide scope for speed | Keep function scope by default; widen only for immutable or read-only data; test order must never matter |
| `assert (value, "message")` is always true | A non-empty tuple is truthy | `assert value, "message"` (Python warns with `SyntaxWarning`; treat warnings as errors) |
| `pytest.raises(Exception)` without `match=` | Passes on any unrelated error | Catch the narrowest type and check `match=` |
| `assert 0.1 + 0.2 == 0.3` | Binary floating point | `pytest.approx` |
| Forgetting to clear `app.dependency_overrides` | The app object outlives the test | Build the app in a fixture and clear (or discard) it after the test |
| `TestClient(app)` without `with` does not run startup/shutdown (lifespan) events | Only the context-manager form runs them | `with TestClient(app) as client:` when the app needs its lifespan |
| Two `test_utils.py` in different folders: `import file mismatch` | Default import mode puts the file's folder on `sys.path`, so basenames must be unique | Unique basenames (this repo prefixes `mNN_`) or `__init__.py` packages / `--import-mode=importlib` |
| An `async def` test that never runs or is reported as unsupported | No async plugin configured | Use the anyio plugin (`anyio_mode = "auto"` in `pyproject.toml`) |
| SQLite `:memory:` data "disappears" or `ProgrammingError` in handler threads | Each connection has its own memory DB; sync FastAPI handlers run in worker threads | Share one connection (`check_same_thread=False`) or use a file in `tmp_path` |
| `time.sleep()` or `datetime.now()` in tests | Easy to write | Inject a clock and advance it; never sleep in unit tests |
| Asserting on every internal call | Tests mirror the implementation | Assert on results and visible effects; verify interactions only when they *are* the behaviour |

## 5. Coming From Other Languages

- **Go:** table-driven tests become `@pytest.mark.parametrize`; `httptest.NewRecorder` is `TestClient`; `t.Cleanup` is `yield` in a fixture; `testing.B` benchmarks are `timeit` (or the third-party `pytest-benchmark`, not used here). Go interfaces are satisfied implicitly, and so are Python `Protocol`s: that is what makes fakes easy.
- **Java/C# (JUnit/xUnit, Mockito/Moq):** no test classes or annotations needed; fixtures replace `@BeforeEach`/`[SetUp]`. Mock frameworks exist in the stdlib, but hand-written fakes are more common. DI containers are replaced by plain arguments and `Depends`.
- **JavaScript/TypeScript (Jest, supertest):** `jest.mock` replaces modules by path, like `monkeypatch`/`patch`, with the same "patch where it is used" rule; `supertest(app)` is `TestClient(app)`.
- **Ruby (RSpec):** `let`/`before` ~ fixtures; `allow(...).to receive` ~ `monkeypatch`/`mock`.

## 6. Exercises

Each exercise has starter code and tests in [exercises/](./exercises/). Because the starter must fail, these are *implementation* tasks whose tests demonstrate a testing technique. Solutions are in [solutions/](./solutions/).

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | [Fake clock and expiring cache](./exercises/ex1_fake_clock/m12_fake_clock.py) | Easy | injected clock, deterministic time-driven tests | `pytest modules/12-testing/exercises/ex1_fake_clock` |
| 2 | [In-memory store under a contract](./exercises/ex2_contract_store/m12_store.py) | Medium | contract tests, fakes vs the real thing | `pytest modules/12-testing/exercises/ex2_contract_store` |
| 3 | [App with replaceable collaborators](./exercises/ex3_di_app/m12_tasks_app.py) | Stretch | `Depends`, `dependency_overrides`, spies | `pytest modules/12-testing/exercises/ex3_di_app` |

**Exercise 1: Fake clock and expiring cache**

- **Task:** implement `FakeClock` (aware datetimes only, `advance`, `set`, never backwards) and `ExpiringCache` (ttl, expiry exactly at the ttl, `len` of live entries, `purge`).
- **Acceptance:** all tests in `test_m12_fake_clock.py`; note that no test sleeps.
- <details><summary>Hint</summary>Store each entry's expiry time (`clock() + ttl`) and compare it with `clock()` on every read.</details>

**Exercise 2: In-memory store under a contract**

- **Task:** implement `InMemoryTaskStore` so the shared contract passes for both it and the provided `SqliteTaskStore`: sequential ids that are never reused, ordered and paginated listing, status validation, `None`/`False` for unknown ids.
- **Acceptance:** every test in `test_m12_store.py` passes for `[memory]` and `[sqlite]`. Read the failing `[memory]` cases first: they spell out the contract.
- <details><summary>Hint</summary>`len(self._tasks) + 1` breaks after a delete; keep a separate counter. Validate the status *before* looking the task up.</details>

**Exercise 3: App with replaceable collaborators**

- **Task:** implement `create_app()` with three routes that obtain the store and clock only via `Depends(get_store)` and `Depends(get_clock)`.
- **Acceptance:** all tests in `test_m12_tasks_app.py`: they override both dependencies with a spy store and a frozen clock, and check that the real store stays untouched.
- <details><summary>Hint</summary>`Annotated[TaskStore, Depends(get_store)]` as a parameter type; set the `Location` header through a `Response` parameter.</details>

## 7. Recap

- Make code testable by **injecting** collaborators (clock, store, notifier); then plain fakes are enough.
- pytest: plain `assert`, `parametrize` for tables, fixtures for setup/teardown, `monkeypatch`/`tmp_path`/`caplog` for the environment.
- Mocks must be `autospec`ed; patch the name where it is looked up; prefer fakes for your own code.
- Test HTTP in-process with `TestClient` and `dependency_overrides`; run contract tests against every implementation and against a real SQLite database.
- Use coverage to find gaps, not as a score; benchmark only with a question in mind.

## 8. Self-check Questions

1. Why does `monkeypatch.setattr(datetime, "now", ...)` not freeze time for code that did `from datetime import datetime`, and what is the cleaner design?
2. Which bugs does `create_autospec` catch that `Mock()` hides? Give one example.
3. Your fake repo passes all tests but the SQLite one fails in production on "id reuse". What kind of test would have caught it?
4. When is it safe to use a `scope="module"` fixture?
5. 100 % coverage and a green suite: name two bugs that can still exist.

## 9. Further Reading

- [pytest documentation](https://docs.pytest.org/): fixtures, parametrization, monkeypatch, good practices
- [unittest.mock](https://docs.python.org/3/library/unittest.mock.html) (see "Where to patch") and [FastAPI: Testing](https://fastapi.tiangolo.com/tutorial/testing/) / [Testing dependencies with overrides](https://fastapi.tiangolo.com/advanced/testing-dependencies/)
- [Coverage.py](https://coverage.readthedocs.io/) and [`timeit`](https://docs.python.org/3/library/timeit.html)

---
**Next:** [13 Time and Standard Library Utilities](../13-time-and-stdlib-utilities/README.md)
