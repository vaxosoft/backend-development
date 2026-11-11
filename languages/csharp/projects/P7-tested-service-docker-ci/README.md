# P7: Tested Service with Docker and CI

Take a small Tasks service and make it *operable*: validated configuration, health and readiness probes, a metrics endpoint, a production `Dockerfile` and a CI pipeline, all backed by tests.

## Goal

The starter already serves tasks. You add what turns a demo into something you could deploy: fail-fast configuration, `/readyz`, `/metrics`, a multi-stage non-root `Dockerfile`, a `.dockerignore` and a GitHub Actions workflow. Docker is **not** run by the tests: "repo hygiene" tests read the files and assert the rules.

## Requirements

**Service** (already in the starter, `Tasks.cs`): in-memory store behind `ITaskStore`.

| Endpoint | Behaviour |
|----------|-----------|
| `POST /tasks` | `{"title"}` (trimmed, 1-200 chars) -> `201` + `Location: /tasks/{id}` + `{"id","title","status":"todo","created_at"}`; invalid title `422` problem details; malformed JSON `400` |
| `GET /tasks/{id}` | `200` or `404` |
| `GET /tasks` | `200` `{"items":[...],"total":N}` |

**Operations** (your work):

| Endpoint | Behaviour |
|----------|-----------|
| `GET /healthz` | `200` `{"status":"ok","env":"<ENV>"}` (provided) |
| `GET /readyz` | `200` `{"status":"ready"}` once the app has started; `503` `{"status":"not_ready"}` before that and again while stopping |
| `GET /metrics` | `200` `text/plain`: `requests_total <n>` + newline. Counts every request entering the pipeline, including the `/metrics` request itself and 404s |

**Configuration** from the environment, parsed by `AppConfig.Parse(Func<string, string?> get)` which returns a `ConfigResult` (`Config` or the list of `Errors`):

| Variable | Valid values | Default |
|----------|--------------|---------|
| `PORT` | integer 1-65535 | `8080` |
| `LOG_LEVEL` | `debug`, `info`, `warn`, `error` (case-insensitive) | `info` |
| `ENV` | `dev`, `staging`, `prod` (case-insensitive) | `dev` |

Unset or blank means default. Every problem is collected (not just the first) and each message names the variable and the bad value. `Program.cs` prints the errors to stderr and exits with code `78`. `ADDR` (`host:port`) overrides the listen address; otherwise the service listens on `0.0.0.0:$PORT`.

**Packaging** (all new files go in `starter/`):

- `Dockerfile`: at least two `FROM` stages; the build stage uses the `dotnet/sdk` image and runs `dotnet publish`; the final stage uses `dotnet/aspnet` (or runtime/runtime-deps), copies the published output with `COPY --from=`, ends with a non-root `USER`, has `EXPOSE`, and either a `HEALTHCHECK` or a comment starting with `Probe` that names `/healthz`. Restore in its own layer before copying the sources; never copy `bin/` or `obj/` from the host.
- `.dockerignore`: at least `bin`, `obj`, `.git`.
- `.github/workflows/ci.yml`: triggers on `push` and `pull_request`; `permissions: contents: read`; `actions/setup-dotnet` pinned to `10.0.x`; steps restore, build with `-warnaserror`, `dotnet format ... --verify-no-changes`, and `dotnet test`.

## Acceptance criteria

| # | Criterion | Tests |
|---|-----------|-------|
| 1 | Tasks API works, store is replaceable | `BehaviourTests.Created_task_is_returned_with_201_and_location_and_can_be_fetched`, `Blank_title_is_422_problem_details`, `Unknown_task_is_404`, `Task_list_comes_from_the_store_behind_the_interface` |
| 2 | Health reports config | `Healthz_reports_ok_and_the_configured_environment` |
| 3 | Readiness | `Readyz_is_200_once_the_application_has_started`, `Readyz_is_503_while_the_service_is_not_ready` |
| 4 | Metrics | `Metrics_counts_every_request_including_the_scrape_itself` |
| 5 | Config parsing | `ConfigTests.*` (7 tests: defaults, blank, case, levels, port, unknown values, all errors at once) |
| 6 | Invalid config stops startup | `Invalid_environment_makes_startup_fail` |
| 7 | Dockerfile rules | `RepoHygieneTests.Dockerfile_*` (5 tests) |
| 8 | `.dockerignore` rules | `Dockerignore_keeps_build_output_and_git_history_out_of_the_build_context` |
| 9 | CI rules | `RepoHygieneTests.Ci_*` (3 tests) |

## Run

From `languages/csharp/`:

```bash
export DOTNET_NOLOGO=1 DOTNET_CLI_TELEMETRY_OPTOUT=1
PORT=8080 LOG_LEVEL=info ENV=dev dotnet run --project projects/P7-tested-service-docker-ci/starter
```

With the solution applied:

```bash
curl -s localhost:8080/healthz          # {"status":"ok","env":"dev"}
curl -s localhost:8080/readyz           # {"status":"ready"}
curl -s localhost:8080/metrics          # requests_total 3
PORT=abc LOG_LEVEL=loud dotnet run --project projects/P7-tested-service-docker-ci/starter
# Invalid configuration:
#   - PORT must be an integer between 1 and 65535 (got 'abc')
#   - LOG_LEVEL must be one of debug, info, warn, error (got 'loud')
```

Docker (not available on every machine, so never part of the tests): from `starter/`, `docker build -t tasks-service . && docker run --rm -p 8080:8080 tasks-service`.

## Tests

```bash
dotnet test projects/P7-tested-service-docker-ci/tests
```

Each test builds its own in-process app (`ServiceApp` in `tests/`) so counters and readiness never leak between tests; settings passed to it play the role of environment variables. The hygiene tests locate `starter/` relative to the test binary and fail with a readable message when a file is missing. On the starter the Tasks tests pass and the operations, config and hygiene tests fail.

| File | TODO |
|------|------|
| `AppConfig.cs` | `Parse` |
| `Operations.cs` | `UseRequestCounting` middleware, `/readyz`, `/metrics` |
| `Program.cs` | mark readiness on `ApplicationStarted` / `ApplicationStopping` |
| new: `Dockerfile`, `.dockerignore`, `.github/workflows/ci.yml` | packaging |

## Steps and hints

1. Config first: pure function, fastest feedback (`ConfigTests`).
2. Operations: counter middleware, `/readyz`, `/metrics`, readiness hooks.
3. Dockerfile, `.dockerignore`, `ci.yml`; run the hygiene tests after each file.

<details>
<summary>Hint: parsing</summary>

`int.TryParse(raw, NumberStyles.None, CultureInfo.InvariantCulture, out var port)` rejects `-1` and `80.5` for you; then check the range. `Enum`-style parsing is tempting for `LOG_LEVEL`, but `info`/`warn` are not `LogLevel` names, so use a `switch`.
</details>

<details>
<summary>Hint: ready only after startup</summary>

`app.Lifetime.ApplicationStarted.Register(readiness.MarkReady)`. Orchestrators send traffic once `/readyz` is 200; marking not-ready on `ApplicationStopping` lets them drain the instance before it exits.
</details>

<details>
<summary>Hint: Dockerfile</summary>

```dockerfile
FROM mcr.microsoft.com/dotnet/sdk:10.0 AS build
# the restore layer stays cached until the project file changes
COPY P7.TestedService.csproj ./
RUN dotnet restore
COPY . .
RUN dotnet publish -c Release -o /app --no-restore

FROM mcr.microsoft.com/dotnet/aspnet:10.0
COPY --from=build /app .
# the official image provides this unprivileged user
USER app
```

The aspnet image has no `curl`, so a `HEALTHCHECK` would need extra packages; the starting point documents the probe for the orchestrator instead.
</details>

<details>
<summary>Hint: where does CI run?</summary>

GitHub only reads workflows from the repository root. The file lives in `starter/.github/` so it travels with the project; when you copy `starter/` to the root of your own repository, put the tests project in `tests/` (and point its `ProjectReference` at `../P7.TestedService.csproj`).
</details>

## Modules exercised

- [09 Configuration](../../modules/09-configuration/README.md): environment variables, validation, fail fast
- [10 Logging and diagnostics](../../modules/10-logging-and-diagnostics/README.md): log level from config, health/readiness/metrics
- [12 Testing](../../modules/12-testing/README.md): `WebApplicationFactory<Program>`, replacing services, NSubstitute, one app per test
- [14 Packaging and running](../../modules/14-packaging-and-running/README.md): multi-stage Dockerfile, non-root user, CI

## Stretch goals (not tested)

- Add `HEALTHCHECK` using a tiny `--healthcheck` flag of your own app (no curl needed).
- Cache NuGet packages in CI (`actions/setup-dotnet` `cache: true`) and upload the test results as an artifact.
- Publish the image from CI on tags only.
- Switch `/metrics` to the Prometheus text format with a `# TYPE requests_total counter` line and per-status counters via `System.Diagnostics.Metrics`.

## Reference solution

`solution/` holds the files that differ from `starter/` (`AppConfig.cs`, `Operations.cs`, `Program.cs`, `Dockerfile`, `.dockerignore`, `.github/workflows/ci.yml`). Copy them over `starter/` to compare: `cp -r projects/P7-tested-service-docker-ci/solution/. projects/P7-tested-service-docker-ci/starter/` (this overwrites your work).
