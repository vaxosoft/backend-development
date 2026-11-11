# P7 - Tested Service with Docker and CI

Take a small Tasks service and make it production-shaped: validated configuration, health/readiness/metrics endpoints, request logging, a multi-stage non-root `Dockerfile`, a `.dockerignore` and a GitHub Actions workflow. Tests check behaviour **and** the packaging files. Docker itself is not run by the tests.

## Goal

Finish the service in `starter/` and add the packaging files so every test passes.

## Requirements

Already in the starter: `POST /tasks`, `GET /tasks`, `GET /tasks/{id}`, `GET /healthz`, an in-memory `tasks.Store` behind an interface, the error envelope and `cmd/server` (reads `ADDR`, handles Ctrl-C/SIGTERM, supports `-healthcheck`).

**You add:**

| Area | Requirement |
|------|-------------|
| `GET /readyz` | `503` with envelope code `unavailable` until `Service.SetReady(true)`; then `200 {"status":"ready"}`. `SetReady(false)` flips it back. |
| `GET /metrics` | `200`, `text/plain`, body `requests_total <n>\n`. `n` counts **every** request (any route, any status), including the current one. |
| Request logging | One `slog` line per request, message `request`, attributes `method`, `path`, `status`, `duration_ms`. |
| `LoadConfig(getenv)` | `PORT` (default 8080, 1-65535), `LOG_LEVEL` (default `info`; `debug\|info\|warn\|error`, case-insensitive), `ENV` (default `development`; `development\|test\|production`, case-insensitive). Empty = default. Invalid values: return an error wrapping `ErrInvalidConfig` that **names the variable**; report all problems together (`errors.Join`). |
| `starter/Dockerfile` | Multi-stage (>= 2 `FROM`); first stage runs `go mod download` before `COPY . .`, then `CGO_ENABLED=0 go build`; final stage is **not** a `golang` image, uses `COPY --from=`, sets a non-root `USER`, `EXPOSE`s the port and has a `HEALTHCHECK` (or a documented `/healthz` probe). |
| `starter/.dockerignore` | Excludes at least `.git` and `bin`. |
| `starter/.github/workflows/ci.yml` | Triggers on `push` and `pull_request`; steps for `go build`, `gofmt -l`, `go vet`, `go test`; `actions/setup-go` pinned via `go-version-file` (pointing at `go.mod`) or a `1.27.x` version. |

Errors on the Tasks API: malformed JSON `400 bad_request`, invalid task `422 validation_failed` (title 1-200 trimmed characters, status `todo|in_progress|done`, default `todo`), unknown id `404 not_found`, store failure `500 internal` (no details leaked). The list response is simplified to `{"items","total"}` (no paging).

## Acceptance criteria

| Criterion | Test (`tests/`) |
|-----------|-----------------|
| Health | `TestHealthzReportsOK` |
| Readiness lifecycle | `TestReadyzIs503UntilServiceIsMarkedReady`, `TestReadyzIs200AfterSetReady`, `TestReadyzGoesBackTo503WhenReadinessIsRevoked` |
| Metrics | `TestMetricsCountsEveryRequestIncludingItself`, `TestMetricsCountsUnknownRoutesAndErrors` |
| Request logging | `TestEachRequestIsLoggedWithMethodPathAndStatus` |
| Tasks API (given) | `TestCreateTaskThenFetchIt`, `TestCreateTaskValidationReturns422WithFields`, `TestCreateTaskWithMalformedJSONReturns400`, `TestUnknownTaskReturns404`, `TestListTasksReturnsItemsInCreationOrder`, `TestStoreFailureReturns500WithoutLeakingDetails` |
| Config | `TestLoadConfigUsesDefaultsWhenNothingIsSet`, `TestLoadConfigReadsTheEnvironment`, `TestLoadConfigIgnoresCaseOfLogLevelAndEnv`, `TestLoadConfigRejectsInvalidValues`, `TestLoadConfigReportsEveryProblemAtOnce` |
| Dockerfile | `TestDockerfileIsMultiStage`, `...BuildsInABuildStageWithoutCgo`, `...CachesModuleDownloadsBeforeCopyingSources`, `...FinalStageIsMinimalAndCopiesTheBinary`, `...RunsAsNonRoot`, `...ExposesThePort`, `...HasAHealthcheckOrDocumentedProbe` |
| `.dockerignore` | `TestDockerignoreExcludesGitHistoryAndBuildOutput` |
| CI | `TestCITriggersOnPushAndPullRequest`, `TestCIRunsBuildFormatLintAndTests`, `TestCIUsesThePinnedToolchain` |

## Running

From `languages/go/`:

```bash
ADDR=127.0.0.1:8080 ENV=development LOG_LEVEL=debug go run ./projects/P7-tested-service-docker-ci/starter/cmd/server
curl localhost:8080/readyz
curl -X POST localhost:8080/tasks -d '{"title":"Write docs"}'
curl localhost:8080/metrics
```

| Variable | Default | Meaning |
|----------|---------|---------|
| `PORT` | `8080` | port, used when `ADDR` is unset (binds to `127.0.0.1`) |
| `ADDR` | `127.0.0.1:<PORT>` | full `host:port`; wins over `PORT` (the Docker image sets `0.0.0.0:8080`) |
| `LOG_LEVEL` | `info` | `debug\|info\|warn\|error` |
| `ENV` | `development` | `production` switches logs to JSON |

Invalid values stop the process with exit code 2 and a message naming the variable, e.g. `PORT=abc`.

Docker (not run by the tests; needs Docker on your machine). The build context is the Go module root so `go.mod` is visible:

```bash
docker build -f projects/P7-tested-service-docker-ci/starter/Dockerfile -t tasks-service .
docker run --rm -p 8080:8080 tasks-service
```

Docker reads `.dockerignore` from the context root, so for a real build copy it to `languages/go/.dockerignore` (or name it `Dockerfile.dockerignore` next to the Dockerfile with BuildKit). Copy `ci.yml` to `.github/workflows/` at the repository root for GitHub to run it.

## Running the tests

```bash
go test ./projects/P7-tested-service-docker-ci/tests/
```

The Tasks tests pass on the starter; the readiness, metrics, logging, config and packaging tests fail until you do the work.

## Suggested steps

1. `config.go`: implement `LoadConfig` (table-driven thinking: one `if` per variable).
2. `server.go`: add `/readyz`, `/metrics`, then the instrumenting middleware.
3. Write `Dockerfile` and `.dockerignore`.
4. Write `.github/workflows/ci.yml`.
5. Run the service and `curl` it; try `PORT=abc`.

<details>
<summary>Hints</summary>

- Readiness is an `atomic.Bool` (`s.ready`); the request counter is an `atomic.Int64` (`s.requests`).
- Middleware: `func (s *Service) instrument(next http.Handler) http.Handler`. Increment the counter **before** calling `next` so `/metrics` counts itself. Wrap the `ResponseWriter` to capture the status.
- `slog.Level.UnmarshalText` already understands the level names; a `[]string` plus `slices.Contains` validates them.
- `fmt.Errorf("%w: PORT %q must be ...", ErrInvalidConfig, v)` wraps and names the variable; `errors.Join` combines several errors.
- Pin the Go version in CI with `go-version-file: languages/go/go.mod` so the workflow and the module never drift.
- A distroless image has no shell or `curl`: the starter's `main` supports `-healthcheck` (`HEALTHCHECK CMD ["/server","-healthcheck"]`).
- Static binary + `gcr.io/distroless/static:nonroot` (or `scratch` with a numeric `USER`) is the usual minimal runtime.

</details>

## Modules exercised

- [09 Configuration](../../modules/09-configuration/README.md): environment config with validation
- [10 Logging and diagnostics](../../modules/10-logging-and-diagnostics/README.md): `slog`, health/readiness/metrics
- [12 Testing](../../modules/12-testing/README.md): black-box tests over `httptest`, a fake `Store`
- [14 Packaging and running](../../modules/14-packaging-and-running/README.md): Docker, CI, non-root runtime

## Stretch goals (not tested)

- Count requests per route and status in `/metrics` (Prometheus text format).
- Add `-version` output through `debug.ReadBuildInfo`.
- Run `go test -race -cover` in CI and upload the coverage profile.
- Add a `Makefile` or `Taskfile` that wraps build, test and `docker build`.

## Reference solution

[`_solution/`](./_solution/) contains `config.go`, `server.go`, `Dockerfile`, `.dockerignore` and `.github/workflows/ci.yml` at the paths where they belong in `starter/`. Try the exercise first.
