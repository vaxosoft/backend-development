# Backend Development in Go

Learn backend development with Go 1.25+ and its standard library: language constructs and the core packages only (HTTP, no gRPC/GraphQL/brokers). Every lesson uses the same Task Tracker domain described in [../../framework/domain-and-projects.md](../../framework/domain-and-projects.md). Library choices and conventions are in [PROFILE.md](./PROFILE.md).

## Prerequisites
- Programming experience in any language. No Go needed; module 01 covers the language.
- [Go](https://go.dev/dl/) 1.25 or newer (`go version`). The repo is built with 1.27.
- `curl` and an editor with gopls. For `go test -race` you also need a C compiler (`gcc`).

## Setup
All commands run from this folder (`languages/go/`), the root of a single Go module named `webdev`.

```bash
go build ./...                                          # downloads the few dependencies once
go run ./modules/01-language-fundamentals/examples/01_values_and_flow
```

| Path | Contents |
|------|----------|
| `modules/NN-topic/README.md` | the lesson |
| `modules/NN-topic/examples/NN_name/` | runnable `package main` programs: `go run ./modules/02-http-server/examples/01_hello_mux` (module 12 examples are test files: `go test`) |
| `modules/NN-topic/exercises/exN_name/` | starter package + tests (they fail until you solve them) |
| `modules/NN-topic/_solutions/exN_name/` | reference solution files (same names as in the exercise). The leading `_` makes the Go tool ignore the folder |
| `projects/PN-name/` | capstone: `README.md`, `starter/`, `tests/`, `_solution/` |

Working on an exercise:

```bash
go test ./modules/02-http-server/exercises/ex1_routes/    # red
# edit the non-test file in that folder until green
```
Peek at `_solutions/` only after a real attempt. Test a package, not `./...`: `go test ./...` fails by design until every exercise is solved.

Quality commands: `gofmt -l .`, `go vet ./...`, `go test -race <package>`.

## Learning path

| # | Module | Time |
|---|--------|------|
| 01 | [Language fundamentals](./modules/01-language-fundamentals/README.md) | 2h |
| 02 | [HTTP server](./modules/02-http-server/README.md) | 2h |
| 03 | [REST API design](./modules/03-rest-api-design/README.md) | 2h |
| 04 | [Serialization](./modules/04-serialization/README.md) | 1.5-2h |
| 05 | [Database access](./modules/05-database-access/README.md) | 2h |
| 06 | [Concurrency](./modules/06-concurrency/README.md) | 2h |
| 07 | [HTTP client](./modules/07-http-client/README.md) | 1.5-2h |
| 08 | [File and stream I/O](./modules/08-file-and-stream-io/README.md) | 1.5-2h |
| 09 | [Configuration](./modules/09-configuration/README.md) | 1.5h |
| 10 | [Logging and diagnostics](./modules/10-logging-and-diagnostics/README.md) | 1.5-2h |
| 11 | [Auth and security basics](./modules/11-auth-and-security-basics/README.md) | 2h |
| 12 | [Testing](./modules/12-testing/README.md) | 2h |
| 13 | [Time and standard library utilities](./modules/13-time-and-stdlib-utilities/README.md) | 2h |
| 14 | [Packaging and running](./modules/14-packaging-and-running/README.md) | 1.5-2h |

Follow the order: modules 02-03 set up the handler and error conventions that later modules reuse, and 06 (concurrency) is needed for projects P4 and P6.

## Capstone projects
Each project has a README with requirements and acceptance criteria. Make the tests in `tests/` pass by completing `starter/`.

| Project | Exercises modules | Run the tests |
|---------|-------------------|---------------|
| [P1 Hello HTTP](./projects/P1-hello-http/README.md) | 01, 02, 10 | `go test ./projects/P1-hello-http/tests/` |
| [P2 Tasks CRUD API](./projects/P2-tasks-crud-api/README.md) | 02-05 | `go test ./projects/P2-tasks-crud-api/tests/` |
| [P3 File service](./projects/P3-file-service/README.md) | 02, 08 | `go test ./projects/P3-file-service/tests/` |
| [P4 API aggregator](./projects/P4-api-aggregator/README.md) | 06, 07 | `go test ./projects/P4-api-aggregator/tests/` |
| [P5 URL shortener](./projects/P5-url-shortener/README.md) | 03, 05, 10, 11 | `go test ./projects/P5-url-shortener/tests/` |
| [P6 Job worker pool](./projects/P6-job-worker-pool/README.md) | 06, 10, 13 | `go test ./projects/P6-job-worker-pool/tests/` |
| [P7 Tested service, Docker, CI](./projects/P7-tested-service-docker-ci/README.md) | 09, 10, 12, 14 | `go test ./projects/P7-tested-service-docker-ci/tests/` |

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
- Docker is described in module 14 and project P7 but is not needed to run any test.
- PostgreSQL appears only as a compile-only section in module 05 (it runs only when `DATABASE_URL` is set).
- `modules/00-deps` only pins third-party dependencies; ignore it.
- Maintainers: `python3 ../../scripts/verify.py go` verifies every exercise (starter fails, solution passes), gofmt, vet, and builds.
