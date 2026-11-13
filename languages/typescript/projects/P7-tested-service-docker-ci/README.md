# P7 - Tested Service with Dockerfile and CI

Take a small Tasks service and make it **operable**: readiness and metrics endpoints, validated configuration, a multi-stage non-root Dockerfile and a CI workflow. The tests check the behaviour and also read your packaging files and enforce the rules ("repo hygiene" tests). Docker itself is never run by the tests.

## Goal

- Add the operational endpoints a platform needs (`/readyz`, `/metrics`).
- Fail fast on bad configuration with a function that returns a result you can test.
- Write a production-grade `Dockerfile`, `.dockerignore` and GitHub Actions workflow.

## Modules exercised

- [09 Configuration](../../modules/09-configuration/README.md): env vars are strings, validation, fail fast
- [10 Logging and diagnostics](../../modules/10-logging-and-diagnostics/README.md): readiness, counters, startup logging
- [12 Testing](../../modules/12-testing/README.md): injected store and clock, `inject()`, one app per test
- [14 Packaging and running](../../modules/14-packaging-and-running/README.md): multi-stage builds, non-root, CI

## Requirements

Already in the starter: `GET /healthz`, `POST /tasks`, `GET /tasks` (`limit` default 20, clamped to 100, `offset`), `GET /tasks/{id}`, the in-memory store behind the `TaskStore` interface, the shared error envelope, and `main.ts`.

| Endpoint / unit   | Requirement                                                                                                                                                         |
| ----------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET /readyz`     | `503 {"status":"not_ready"}` until `app.readiness.markReady()` is called, then `200 {"status":"ready"}`; `markNotReady()` flips it back (used when shutdown starts) |
| `GET /metrics`    | `200`, `text/plain`, body `requests_total <n>\n`; `n` counts every request the app received, including the `/metrics` request itself; one counter per app instance  |
| `loadConfig(env)` | returns `{ ok: true, value }` or `{ ok: false, errors }`, never throws                                                                                              |

`loadConfig` reads:

| Variable    | Rule                                          | Default          |
| ----------- | --------------------------------------------- | ---------------- |
| `ADDR`      | `host:port`                                   | `127.0.0.1:8080` |
| `PORT`      | integer 0-65535, overrides the port of `ADDR` | from `ADDR`      |
| `LOG_LEVEL` | `debug`, `info`, `warn` or `error`            | `info`           |
| `ENV`       | `development`, `test` or `production`         | `development`    |

Empty strings count as unset. All problems are collected in `errors`; each message names the variable and the bad value, e.g. `PORT must be an integer from 0 to 65535, got "abc"`. `main.ts` prints them and exits with status 1.

Packaging files (paths relative to `starter/`):

| File                       | Rules checked by the tests                                                                                                                                                                                                                                                                                                               |
| -------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Dockerfile`               | at least two `FROM`; the first stage compiles with `tsc`; the last stage only `COPY --from=<build>`s the output and installs production dependencies only (`npm ci --omit=dev`); base images carry a tag (not `latest`) and the final one is `slim`/`alpine`/`distroless`; final stage has `USER` (not root), `EXPOSE` and `HEALTHCHECK` |
| `.dockerignore`            | lists at least `node_modules`, `.git` and `.env*`                                                                                                                                                                                                                                                                                        |
| `.github/workflows/ci.yml` | triggers on `push` and `pull_request`; steps for `npm ci`, `tsc`, `biome format` (no `--write`), `biome lint` and `node --test`; `actions/setup-node` with a numeric `node-version` equal to the Node major of the Dockerfile base image                                                                                                 |

The Dockerfile and workflow assume `starter/` is the root of its own repository, next to a `package.json`, `package-lock.json`, `tsconfig.json` and `biome.json` (copy them from `languages/typescript/` and trim `package.json` to `fastify`, `pino` and `zod`; the course's `package.json` lists dependencies for every lesson). Docker is not needed to pass the tests.

## What to implement

| File                                                                              | Task                                                                 |
| --------------------------------------------------------------------------------- | -------------------------------------------------------------------- |
| [`starter/src/config.ts`](./starter/src/config.ts)                                | `loadConfig` (the stub returns defaults and ignores the environment) |
| [`starter/src/app.ts`](./starter/src/app.ts)                                      | `onRequest` counter hook, `GET /readyz`, `GET /metrics`              |
| `starter/Dockerfile`, `starter/.dockerignore`, `starter/.github/workflows/ci.yml` | create them                                                          |

## Run

From `languages/typescript/`:

```bash
ADDR=127.0.0.1:8080 node projects/P7-tested-service-docker-ci/starter/src/main.ts
```

```bash
curl -i localhost:8080/readyz
curl -i -X POST localhost:8080/tasks -H 'content-type: application/json' -d '{"title":"Write docs","tags":["docs"]}'
curl localhost:8080/tasks/1
curl localhost:8080/metrics
```

An invalid environment stops the process before it listens:

```bash
PORT=abc LOG_LEVEL=verbose node projects/P7-tested-service-docker-ci/starter/src/main.ts
```

## Test

```bash
node --test "projects/P7-tested-service-docker-ci/tests/**/*.test.ts"
```

`service.test.ts` and `config.test.ts` run in-process (`app.inject()`, no sockets); `hygiene.test.ts` parses the three packaging files from `starter/`. Without them the starter fails with messages such as `starter/Dockerfile is missing or has no FROM`.

## Acceptance criteria

| Criterion                                                                                | Test                                                                                                                                                                                                             |
| ---------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Health, tasks API, envelope (already working)                                            | `GET /healthz answers 200 ...`, `POST /tasks creates a task ...`, `GET /tasks/{id} returns ...`, `... answers 404 ...`, `POST /tasks rejects an invalid task ...`, `the app talks to the injected TaskStore ...` |
| Readiness                                                                                | `GET /readyz is 503 until the app is marked ready`, `GET /readyz goes back to 503 when shutdown starts`                                                                                                          |
| Metrics                                                                                  | `GET /metrics is plain text and counts every request including itself`, `each app instance has its own request counter`                                                                                          |
| Config defaults, values, `PORT` precedence                                               | `loadConfig falls back ...`, `loadConfig reads ADDR, LOG_LEVEL and ENV ...`                                                                                                                                      |
| Config validation                                                                        | `loadConfig rejects a PORT ...`, `loadConfig reports every invalid variable at once`                                                                                                                             |
| Multi-stage build, non-root, `EXPOSE`, `HEALTHCHECK`, production deps, pinned slim bases | the seven `Dockerfile` tests in `hygiene.test.ts` (from `the Dockerfile is multi-stage` to `base images are pinned ...`)                                                                                         |
| `.dockerignore`                                                                          | `.dockerignore keeps node_modules, git data and env files out of the build context`                                                                                                                              |
| CI triggers, steps, pinned Node                                                          | the three `CI workflow` tests                                                                                                                                                                                    |

## Suggested steps

1. `app.ts`: add the hook and the two routes; run the service tests.
2. `config.ts`: write one small helper per variable and push messages into an `errors` array.
3. Write the `Dockerfile` (build stage, runtime stage), then `.dockerignore`, then `ci.yml`; rerun the hygiene tests after each file.

<details>
<summary>Hint: counting requests</summary>

`app.addHook('onRequest', async () => { app.metrics.countRequest(); })` runs before the route handler, so `/metrics` sees its own request in the total. The `Metrics` instance is created inside `buildApp`, which is why two apps never share a counter.

</details>

<details>
<summary>Hint: config as a result, not an exception</summary>

Return a discriminated union (`{ ok: true, value } | { ok: false, errors }`) so callers must look at `ok` before touching `value`. For `LOG_LEVEL` and `ENV`, search the `as const` arrays (`LOG_LEVELS.find(...)`) instead of casting the string. `Number('')` is `0` and `Number('80.5')` is not an integer: check with `/^\d+$/` before converting.

</details>

<details>
<summary>Hint: the Dockerfile skeleton</summary>

```dockerfile
FROM node:26-slim AS build
# copy manifests, npm ci, copy src, tsc --outDir dist ...
FROM node:26-slim AS runtime
# npm ci --omit=dev, COPY --from=build /app/dist ./dist, USER node, EXPOSE, HEALTHCHECK, CMD
```

`tsc` needs `--rewriteRelativeImportExtensions` so the `.ts` extensions in imports become `.js` in the output. The `node` image already ships a non-root `node` user.

</details>

## Stretch goals (not tested)

- Make the final stage even smaller or compare `-slim` with `-alpine` (mind native modules such as `better-sqlite3`).
- Add a `docker build` and image smoke test job (run the container, probe `/readyz`) to the workflow.
- Cache `~/.npm` between CI runs and add a `strategy.matrix` for two Node versions.
- Expose a `requests_total` per status code in Prometheus text format.

## Reference solution

The files that differ (including the three packaging files) are in [`solution/`](./solution/) with the same relative paths as in `starter/`. Copy them over `starter/` to see all tests pass.
