# Backend Development in Python

Learn backend development with Python 3.14 and FastAPI: language constructs and the core libraries only (HTTP, no gRPC/GraphQL/brokers). All code is type-hinted and checked with `mypy --strict`. Every lesson uses the same Task Tracker domain described in [../../framework/domain-and-projects.md](../../framework/domain-and-projects.md). Library choices and conventions are in [PROFILE.md](./PROFILE.md).

## Prerequisites
- Programming experience in any language. No Python needed; module 01 covers the language, type hints and dataclasses.
- [Python](https://www.python.org/downloads/) 3.14 (`python --version`) with `venv` and `pip`.
- `curl` and an editor with Python support (VS Code with the Python and Pylance extensions).

## Setup
All commands run from this folder (`languages/python/`).

```bash
python -m venv .venv
source .venv/bin/activate            # Windows: .venv\Scripts\activate
pip install -r requirements.txt      # pinned versions
python modules/01-language-fundamentals/examples/01_values_and_references.py
```

| Path | Contents |
|------|----------|
| `modules/NN-topic/README.md` | the lesson |
| `modules/NN-topic/examples/` | runnable examples: `python modules/02-http-server/examples/01_stdlib_server.py` |
| `modules/NN-topic/exercises/exN_name/` | starter module plus `test_*.py` (the tests fail until you solve it) |
| `modules/NN-topic/solutions/exN_name/` | reference solution files (same names as in the exercise) |
| `projects/PN-name/` | capstone: `README.md`, `starter/`, `tests/`, `solution/` |

Working on an exercise:

```bash
pytest modules/02-http-server/exercises/ex1_routes      # red
# edit the non-test .py file in that folder until green
```

Peek at `solutions/` only after a real attempt. Run `pytest` on one exercise folder, not the whole repo: the unsolved exercises fail by design.

Quality commands: `ruff format .`, `ruff check .`, and `mypy <folder>` (check one lesson folder at a time; example file names repeat across modules).

## Learning path

| # | Module | Time |
|---|--------|------|
| 01 | [Language fundamentals](./modules/01-language-fundamentals/README.md) | 1.5-2h |
| 02 | [HTTP server](./modules/02-http-server/README.md) | 2h |
| 03 | [REST API design](./modules/03-rest-api-design/README.md) | 2h |
| 04 | [Serialization](./modules/04-serialization/README.md) | 1.5-2h |
| 05 | [Database access](./modules/05-database-access/README.md) | 2h |
| 06 | [Concurrency](./modules/06-concurrency/README.md) | 2h |
| 07 | [HTTP client](./modules/07-http-client/README.md) | 1.5-2h |
| 08 | [File and stream I/O](./modules/08-file-and-stream-io/README.md) | 1.5-2h |
| 09 | [Configuration](./modules/09-configuration/README.md) | 1.5-2h |
| 10 | [Logging and diagnostics](./modules/10-logging-and-diagnostics/README.md) | 1.5-2 h |
| 11 | [Auth and security basics](./modules/11-auth-and-security-basics/README.md) | 2 h |
| 12 | [Testing](./modules/12-testing/README.md) | 2 h |
| 13 | [Time and standard library utilities](./modules/13-time-and-stdlib-utilities/README.md) | 1.5-2 h |
| 14 | [Packaging and running](./modules/14-packaging-and-running/README.md) | 1.5-2 h |

Follow the order: modules 02-03 set up the FastAPI and error-envelope conventions that later modules reuse, and 06 (the GIL, threads and `asyncio`) is needed for projects P4 and P6.

## Capstone projects
Each project has a README with requirements and acceptance criteria. Make the tests in `tests/` pass by completing `starter/`.

| Project | Exercises modules | Run the tests |
|---------|-------------------|---------------|
| [P1 Hello HTTP](./projects/P1-hello-http/README.md) | 01, 02, 10 | `pytest projects/P1-hello-http/tests` |
| [P2 Tasks CRUD API](./projects/P2-tasks-crud-api/README.md) | 02-05 | `pytest projects/P2-tasks-crud-api/tests` |
| [P3 File service](./projects/P3-file-service/README.md) | 02, 08 | `pytest projects/P3-file-service/tests` |
| [P4 API aggregator](./projects/P4-api-aggregator/README.md) | 06, 07 | `pytest projects/P4-api-aggregator/tests` |
| [P5 URL shortener](./projects/P5-url-shortener/README.md) | 03, 05, 10, 11 | `pytest projects/P5-url-shortener/tests` |
| [P6 Job worker pool](./projects/P6-job-worker-pool/README.md) | 06, 10, 13 | `pytest projects/P6-job-worker-pool/tests` |
| [P7 Tested service, Docker, CI](./projects/P7-tested-service-docker-ci/README.md) | 09, 10, 12, 14 | `pytest projects/P7-tested-service-docker-ci/tests` |

## Progress checklist
- [ ] 01 Language fundamentals
- [ ] 02 HTTP server
- [ ] 03 REST API design
- [ ] 04 Serialization
- [ ] 05 Database access
- [ ] 06 Concurrency
- [ ] 07 HTTP client
- [ ] 08 File and stream I/O
- [ ] 09 Configuration
- [ ] 10 Logging and diagnostics
- [ ] 11 Auth and security basics
- [ ] 12 Testing
- [ ] 13 Time and standard library utilities
- [ ] 14 Packaging and running
- [ ] P1 - P7 capstones

## Notes
- Exercise module names are prefixed (`mNN_`) so every file name is unique: pytest imports test files by name.
- Docker is described in module 14 and project P7 but is not needed to run any test.
- PostgreSQL appears only as a compile-only section in module 05 (it connects only when `DATABASE_URL` is set).
- `modules/00-deps-check` only checks the dependency install; ignore it.
- Maintainers: `python3 ../../scripts/verify.py python` verifies every exercise (starter fails, solution passes), formatting, lint, and `mypy --strict`. It expects the venv at `~/.venvs/backend-python` (or set `VERIFY_VENV`).
