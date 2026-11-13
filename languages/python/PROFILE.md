# Language Profile: Python

Constrains all generated Python content. Dependencies are pinned in [requirements.txt](./requirements.txt) (a full `pip freeze` of a clean Python 3.14 venv) and tool settings live in [pyproject.toml](./pyproject.toml). Do not add dependencies while writing a module.

## 1. Overview
- **Version:** Python 3.14. Fully type-annotated code, checked with `mypy --strict`.
- **Paradigm / typing:** dynamically typed with optional static types, garbage collected (reference counting plus cycle collector), multi-paradigm (functions, classes, dataclasses, generators, async). The standard CPython build has a Global Interpreter Lock (GIL); a free-threaded build exists but is not used here.
- **Why backend:** very productive, enormous ecosystem, great for data-heavy and glue services, and FastAPI makes type hints drive validation and documentation. Pydantic-style typed models bring much of the safety of statically typed stacks.

## 2. Toolchain

| Purpose | Tool | Command |
|---------|------|---------|
| Install / version manager | python.org installer, `pyenv`, or `uv python install` | `python --version` (3.14) |
| Virtual environment | `venv` | `python -m venv .venv && source .venv/bin/activate` |
| Package manager | `pip` with the pinned file | `pip install -r requirements.txt` |
| Run an example | `python` | `python modules/02-http-server/examples/01_hello.py` |
| Test | `pytest` | `pytest modules/02-http-server/exercises/ex1_routes` |
| Format | `ruff format` | `ruff format .` |
| Lint | `ruff check` | `ruff check .` (add `--fix` to apply fixes) |
| Type check | `mypy` (strict) | `mypy modules/02-http-server/examples` |
| Debug / profile | `pdb`/`breakpoint()`, `python -X dev`, `cProfile`, `tracemalloc`, `faulthandler` | |

All commands run from `languages/python/` with the venv active. The repo's verification script expects the venv at `~/.venvs/backend-python` (or set `VERIFY_VENV`).

## 3. Project Conventions
- **Layout of this repo:** one `pyproject.toml` and one venv for the whole track. Folders such as `02-http-server` are not importable names, so nothing imports across lesson folders by package path.
  - Examples: `modules/NN-topic/examples/NN_name.py`, run as scripts with `python <path>` (`if __name__ == "__main__":`). A helper used by several examples of one module is a sibling file with a unique name (`mNN_helpers.py`).
  - Exercises: `modules/NN-topic/exercises/exN_name/{mNN_name.py, test_mNN_name.py}`. **Module basenames are unique across the repo** (prefix `mNN_`), because pytest imports test files by basename (default `prepend` import mode adds the test file's folder to `sys.path`, so tests simply `from mNN_name import ...`). Starter code raises `NotImplementedError`, so tests fail on assertions/errors, never on import errors.
  - Solutions: `modules/NN-topic/solutions/exN_name/mNN_name.py` (only non-test files; same relative path).
  - Projects: `projects/PN-name/{README.md, starter/, solution/, tests/}`. `starter/pN_name/` is a package (`__init__.py`, `app.py` with `create_app(config)`, `__main__.py` as the runnable entry). `tests/conftest.py` puts `../starter` on `sys.path`; tests `from pN_name.app import create_app`.
  - `modules/00-deps-check` is infrastructure; do not edit.
- **Naming:** `snake_case` functions, variables, modules; `PascalCase` classes; `UPPER_SNAKE` constants; leading underscore for private; test files `test_*.py`, test functions `test_*`.
- **Types:** annotate every function signature and public attribute (`mypy --strict` must pass, including tests). Use built-in generics (`list[int]`, `dict[str, int]`), `X | None`, `Protocol` for structural interfaces, `dataclass(slots=True, frozen=True)` for plain data, Pydantic models for validated input/output. Python 3.14 evaluates annotations lazily, so no `from __future__ import annotations`.
- **Error handling style:** exceptions. Define a small hierarchy (`class AppError(Exception)`), raise specific errors, `raise ... from err` to keep the cause, catch narrowly, use context managers (`with`) for resources. Expected "not found" outcomes may return `None`/`Optional`. In FastAPI, exception handlers translate domain errors to the shared envelope. Never use a bare `except:` or swallow `Exception`.
- **Docs:** short PEP 257 docstrings on public functions and classes, only where they add information.
- **Style rules enforced by ruff:** import order (isort), pyupgrade (`UP`), bugbear (`B`), simplify (`SIM`), comprehensions (`C4`), pytest style (`PT`). Line length 100.

## 4. Library Choices per Topic
Standard library first. Third-party packages are listed with the reason.

| Module | Library | Version | Why |
|--------|---------|---------|-----|
| 01 Fundamentals | stdlib: `dataclasses`, `enum`, `typing`, `collections`, `itertools`, `functools`, `contextlib`, `pathlib` | 3.14 | |
| 02 HTTP server | `fastapi` on `uvicorn` (ASGI); `starlette` middleware and static files (shipped with FastAPI); `python-multipart` for forms and uploads | fastapi 0.162.5, uvicorn 0.54.0, python-multipart 0.0.33 | FastAPI is the most used modern framework; a stdlib `http.server` example shows what it hides |
| 03 REST design | FastAPI routing, `pydantic` v2 models, exception handlers, built-in OpenAPI at `/openapi.json` | pydantic 2.14.3 | validation, serialization and API docs from one set of type hints |
| 04 Serialization | `json`, `pydantic`, `csv`, `xml.etree.ElementTree`, `tomllib`, `urllib.parse`, `PyYAML` | PyYAML 6.0.3 | everything is in the stdlib except YAML. Always `yaml.safe_load` |
| 05 Database access | `sqlite3` (stdlib) for the main lessons; one `SQLAlchemy` 2.x Core example; `psycopg` for the compile-only PostgreSQL section (runs only if `DATABASE_URL` is set) | SQLAlchemy 2.1.8, psycopg 3.3.9 | `sqlite3` shows the fundamentals (DB-API), SQLAlchemy Core is what teams use for query building. No ORM sessions |
| 06 Concurrency | `threading`, `concurrent.futures`, `asyncio` (`TaskGroup`, `Queue`, `Semaphore`, `timeout`), `multiprocessing` (mention), `contextvars` | stdlib | |
| 07 HTTP client | `httpx` (sync and async), `httpx.MockTransport` for tests, hand-written retry with backoff | httpx 0.28.1 | requests-like API with timeouts and async; `urllib.request` is mentioned |
| 08 File and stream I/O | `pathlib`, `io`, `shutil`, `tempfile`, `gzip`, `zipfile`, `base64`, `hashlib`, `importlib.resources` | stdlib | |
| 09 Configuration | `os.environ`, `argparse`, `tomllib`, `pydantic-settings` | pydantic-settings 2.16.2 | typed settings with env/`.env` loading and validation; layering shown with explicit precedence |
| 10 Logging | `logging` with a small JSON formatter, `contextvars` for request ids, FastAPI/Starlette middleware, `faulthandler`, `cProfile`, `tracemalloc` | stdlib | the stdlib logger is the standard. structlog is mentioned |
| 11 Auth / security | `hashlib.scrypt`, `secrets`, `hmac.compare_digest`, `PyJWT`, FastAPI `Depends` security helpers, hand-written token bucket | PyJWT 2.15.1 | password hashing and constant-time compare are stdlib; JWT is not |
| 12 Testing | `pytest` (fixtures, `parametrize`, `monkeypatch`, `tmp_path`), FastAPI `TestClient`, `unittest.mock`, `pytest-cov`, `timeit`, anyio plugin for async tests (`anyio_mode = "auto"`) | pytest 9.2.0, pytest-cov 7.1.0, anyio 4.16.0 | pytest is the standard runner |
| 13 Time and utilities | `datetime`, `zoneinfo` (+ `tzdata`), `uuid` (`uuid4`, `uuid7`), `random`, `secrets`, `re`, `string.Template`, `asyncio` timers | tzdata 2026.6 | `zoneinfo` needs `tzdata` on systems without an IANA database |
| 14 Packaging | `pyproject.toml`, `pip wheel . --no-deps`, `python -m zipapp`, multi-stage Dockerfile on `python:3.14-slim`, `uvicorn` flags, GitHub Actions | | |

**Database for examples:** SQLite via the stdlib `sqlite3` (`:memory:` or a temp file). PostgreSQL appears only as a documented swap that connects only when `DATABASE_URL` is set and otherwise prints a skip message.

## 5. Language-specific Teaching Notes

| Module | Emphasis | Typical pitfall |
|--------|----------|-----------------|
| 01 | everything is an object, mutability, truthiness, `dataclass`, `Protocol`, comprehensions and generators, closures, context managers, type hints, `Enum`, errors | mutable default arguments, late-binding closures in loops, `is` vs `==`, modifying a list while iterating, shallow vs deep copy |
| 02 | ASGI and uvicorn, path/query/body binding by type hints, `Depends`, middleware order, lifespan events, graceful shutdown | `async def` handler calling blocking code (stalls the loop; use plain `def` or `run_in_threadpool`), returning a Pydantic model vs a dict, forgetting `await` |
| 03 | custom exception handlers for the shared envelope (`RequestValidationError` is 422 by default), `response_model`, pagination bounds with `Query(ge=, le=)`, status codes | leaking internal fields (no `response_model`), different 422 shape than the rest of the API, mutable global state |
| 04 | `model_validate` / `model_dump(mode="json")`, `Optional` vs missing vs `None`, `extra="forbid"`, datetime and timezone-aware values, `json` limitations | `json.dumps` cannot serialise `datetime`/`Decimal`/`set`, naive datetimes, `yaml.load` without a safe loader, silently ignored unknown fields |
| 05 | DB-API connect/cursor/commit, parameter binding (`?`), `row_factory`, transactions as context managers, `sqlite3.Row`, SQLAlchemy Core expressions | f-string SQL, forgetting `commit()` (the connection context manager commits but does not close), `check_same_thread` with threads, `:memory:` per connection |
| 06 | the GIL and what it means for threads vs `asyncio` vs processes, `TaskGroup`, cancellation, `asyncio.timeout`, bounded queues, `run_in_executor` | blocking call in a coroutine, forgotten `await` (a coroutine never runs), `create_task` without keeping a reference, threads for CPU-bound work, `asyncio.run` inside a running loop |
| 07 | `httpx.Client` reuse, timeouts (the default is 5 s but not total time), `raise_for_status`, streaming responses, retry with backoff and jitter | creating a client per request, `timeout=None`, retrying non-idempotent requests, not closing the client |
| 08 | `pathlib`, text vs binary, always pass `encoding="utf-8"`, `with` blocks, chunked reads, `shutil.copyfileobj`, path traversal checks (`resolve()` + `is_relative_to`) | omitted `encoding`, reading a huge file with `read()`, `open` without closing, joining user input into a path |
| 09 | typed settings, precedence defaults < file < env < CLI, `SecretStr`, fail-fast validation | env values are strings (`"false"` is truthy without parsing), reading `os.environ` at call sites, committing `.env` files |
| 10 | `logging` (loggers, handlers, formatters, levels), `extra=` fields, `contextvars` request id, lazy `%s` formatting, exception logging with `exc_info`/`logger.exception` | `logging.basicConfig` in libraries, f-strings in log calls, logging secrets, swallowing tracebacks |
| 11 | scrypt parameters and salt, `hmac.compare_digest`, `secrets` vs `random`, JWT `algorithms=[...]` and `exp`, `Depends`-based auth, bound SQL parameters | `==` for secrets, `random` for tokens, decoding JWT without `algorithms`, long-lived tokens |
| 12 | fixtures and scope, `parametrize`, `monkeypatch`, dependency overrides in FastAPI (`app.dependency_overrides`), `tmp_path`, injecting a clock, `unittest.mock` pitfalls | patching the wrong import path, fixtures leaking state, mocks that accept any call (`autospec`), time-dependent tests |
| 13 | aware vs naive `datetime`, `zoneinfo`, `timedelta`, `uuid7`, `secrets`, compiled `re` patterns, `asyncio` periodic tasks | deprecated `datetime.utcnow()` (naive result) vs `datetime.now(UTC)`, naive datetimes, `time.sleep` in async code, greedy regex backtracking |
| 14 | `pyproject.toml`, virtual envs in containers, slim images, non-root `USER`, `PYTHONUNBUFFERED`, uvicorn host/port/workers, layer caching of `pip install` | installing into the system Python, running as root, `--reload` in production, copying `.venv` into the image |

## 6. Comparison Anchors
JavaScript/TypeScript (async/await, dynamic typing), Java/C# (classes, exceptions, annotations vs decorators), Go (explicit errors vs exceptions, goroutines vs asyncio), Ruby.

## 7. Module Ordering Adjustments
Default order 01-14. Module 01 includes type hints, dataclasses and exceptions before HTTP. Module 06 (the GIL, `asyncio`, threads) is a prerequisite for projects P4 and P6.

## 8. Out of Scope for This Language
Django, Flask and other web frameworks, ORMs with sessions (SQLAlchemy ORM, Django ORM), Celery/RQ, GraphQL, gRPC, metaclasses, C extensions, free-threaded builds, packaging to PyPI, notebooks and data science libraries.
