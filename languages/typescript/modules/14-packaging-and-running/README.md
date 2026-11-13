# 14. Packaging and Running in TypeScript

> Estimated time: 1.5-2h | Difficulty: Intermediate | Prerequisites: [02 HTTP server](../02-http-server/README.md), [09 Configuration](../09-configuration/README.md), [12 Testing](../12-testing/README.md)

## Learning Outcomes

After this module you can:

- [ ] Build a deployable artifact with `tsc` (emit JavaScript, rewrite `.ts` imports) and run it with `node`
- [ ] Write a multi-stage `Dockerfile` for a Node service (production dependencies only, non-root user, exec-form `CMD`) and a `.dockerignore`
- [ ] Describe a CI pipeline (install, typecheck, lint, test, build, image) as GitHub Actions YAML
- [ ] Choose runtime flags and production settings (`NODE_ENV`, `NODE_OPTIONS`, heap size, timeouts) and avoid unsafe ones
- [ ] Shut a service down gracefully on `SIGTERM`, with separate liveness and readiness endpoints

## 1. Concept Overview

Packaging turns source into something a machine you do not control can run: a versioned artifact (a directory, a container image) that contains exactly what the service needs and nothing else. _Running_ it in production adds process concerns: configuration from the environment, signals from the orchestrator, health checks, resource limits and logs on stdout.

A good pipeline is boring: every push runs the same install, typecheck, lint, test and build steps, and the same image that passed CI is the one that is deployed. This machine has no Docker, so the `Dockerfile` and CI YAML in this module are written and explained, and checked as text only; everything else runs.

## 2. How TypeScript Approaches It

- **Development:** Node 24+ runs `.ts` files directly by _type stripping_ (`node src/main.ts`). Types are erased, nothing is compiled, and `tsc` only type-checks (`noEmit`).
- **Production:** build with `tsc` and run plain JavaScript. With `rewriteRelativeImportExtensions` the `import './greeting.ts'` lines that type stripping requires are rewritten to `./greeting.js` in the output. Running `.ts` directly in production is possible (Node >= 24) but Node refuses to strip types for files under `node_modules`, adds start-up work and hides the build from CI.
- **Artifact contents:** `dist/` (compiled code), `package.json` (for `"type": "module"` and the version) and `node_modules` installed with `npm ci --omit=dev`.
- **Cross-platform:** a JavaScript artifact runs anywhere with the right Node version, so there is no cross-compilation; pin the version with `engines` and the base image tag. Native modules (`better-sqlite3`) are the exception: they must be installed on the same OS, libc and CPU architecture as the runtime image. Node's single-executable feature (SEA) exists but is out of scope here.
- **Tools:** `tsc`, `npm`, Docker/BuildKit and GitHub Actions. No bundlers (esbuild, webpack): see [PROFILE](../../PROFILE.md).

Relevant `package.json` fields of a service:

```json
{
  "type": "module",
  "engines": { "node": ">=24" },
  "scripts": {
    "build": "tsc -p tsconfig.build.json",
    "start": "node dist/main.js",
    "dev": "node --watch src/main.ts",
    "test": "node --test \"src/**/*.test.ts\""
  }
}
```

## 3. Guided Examples

### 3.1 What the runtime tells you

- **Goal:** distinguish flags for Node (`execArgv`), arguments for your program (`argv`), `NODE_ENV` and `NODE_OPTIONS`.
- **Run:** `NODE_ENV=production node --max-old-space-size=512 --enable-source-maps modules/14-packaging-and-running/examples/01_runtime_info.ts a b`
- **Code:** [01_runtime_info.ts](examples/01_runtime_info.ts)
- **Walkthrough:** flags before the script name configure the runtime (`--max-old-space-size` raises the V8 heap limit, `--enable-source-maps` maps stack traces of compiled code back to `.ts`), everything after belongs to the program. `NODE_OPTIONS` carries the same flags through the environment, which is how containers set them. Libraries read `NODE_ENV`; only the exact value `production` selects production behaviour.
- **Expected output:**

  ```
  node >= 24 (required by "engines"): true
  runs .ts directly (type stripping): true
  execArgv (flags for node itself): [ '--max-old-space-size=512', '--enable-source-maps' ]
  argv tail (arguments for the program): [ 'a', 'b' ]
  NODE_ENV: production
  NODE_OPTIONS: (unset)
  source maps enabled: true
  heap limit >= 512 MiB: true
  production mode: terse logs, no stack traces in responses
  ```

### 3.2 Build with `tsc`

- **Goal:** emit JavaScript that runs without type stripping.
- **Run:**

  ```bash
  node modules/14-packaging-and-running/examples/02_build_demo/src/main.ts
  npx tsc -p modules/14-packaging-and-running/examples/02_build_demo/tsconfig.build.json
  APP_VERSION=1.2.3 node modules/14-packaging-and-running/examples/02_build_demo/dist/main.js
  rm -r modules/14-packaging-and-running/examples/02_build_demo/dist
  ```
- **Code:** [src/main.ts](examples/02_build_demo/src/main.ts), [src/greeting.ts](examples/02_build_demo/src/greeting.ts), [tsconfig.build.json](examples/02_build_demo/tsconfig.build.json)
- **Walkthrough:** the build config extends the lesson `tsconfig.json`, switches `noEmit` off and adds `rewriteRelativeImportExtensions`, `outDir` and `sourceMap`. `dist/main.js` contains `import { greeting } from './greeting.js'`. The same source runs both ways; `import.meta.url` shows which one is active.

  ```json
  "compilerOptions": { "noEmit": false, "rewriteRelativeImportExtensions": true, "outDir": "dist", "rootDir": "src" }
  ```
- **Expected output:**

  ```
  Hello, packaging! (build dev)
  running as: TypeScript source
  Hello, packaging! (build 1.2.3)
  running as: compiled JavaScript
  ```

### 3.3 Dockerfile and CI, inspected as text

- **Goal:** read the three deployment files and see what a reviewer checks.
- **Run:** `node modules/14-packaging-and-running/examples/03_inspect_deploy_files.ts`
- **Code:** [Dockerfile](examples/Dockerfile), [.dockerignore](examples/.dockerignore), [ci.yml](examples/ci.yml) (would live at `.github/workflows/ci.yml`), [03_inspect_deploy_files.ts](examples/03_inspect_deploy_files.ts)
- **Walkthrough:** the Dockerfile has three stages on the same pinned `node:26-slim` base: `build` (all dependencies, runs `npm run build`), `prod-deps` (`npm ci --omit=dev`, so native modules are installed for the runtime's libc and architecture) and `runtime` (copies only `node_modules` and `dist`, sets `NODE_ENV=production` and the build metadata (`--build-arg APP_VERSION=... --build-arg GIT_COMMIT=...`), drops to `USER node`). `package.json` and the lock file are copied before the sources so the dependency layer stays cached. `CMD ["node", ...]` in exec form makes Node PID 1 so `SIGTERM` reaches it. The workflow caches npm, runs `npm ci`, typecheck, lint, tests and the build on Node 24 and 26, then builds the image without pushing.

  ```dockerfile
  USER node
  CMD ["node", "--enable-source-maps", "dist/main.js"]
  ```
- **Expected output:**

  ```
  stage build     node:26-slim   WORKDIR COPY RUN COPY COPY RUN
  stage prod-deps node:26-slim   WORKDIR COPY RUN
  stage runtime   node:26-slim   ARG ARG ENV WORKDIR COPY COPY COPY USER EXPOSE HEALTHCHECK CMD
  final stage runs as non-root: true
  CMD in exec form: true
  job verify: needs=- steps=7 run=[npm ci | npm run typecheck | npm run lint | npm test | npm run build]
  job docker: needs=verify steps=3 run=[]
  dockerignore excludes node_modules and .env: true
  ```

### 3.4 Graceful shutdown

- **Goal:** finish in-flight requests on `SIGTERM`, refuse new ones, then exit 0.
- **Run:** `ADDR=127.0.0.1:22400 node modules/14-packaging-and-running/examples/04_graceful_shutdown.ts`, then in another terminal:

  ```bash
  curl -s http://127.0.0.1:22400/healthz
  curl -s "http://127.0.0.1:22400/slow?ms=1500" &   # still running when the signal arrives
  kill -TERM <server pid>                           # or press Ctrl-C in the server terminal
  curl -s http://127.0.0.1:22400/healthz            # connection refused (curl exit code 7)
  ```
- **Code:** [04_graceful_shutdown.ts](examples/04_graceful_shutdown.ts), [_shared.ts](examples/_shared.ts) (`runService`)
- **Walkthrough:** `runService` listens, waits for `SIGINT`/`SIGTERM`, then calls `app.close()`, which stops accepting connections and resolves when active requests are done. While draining, responses carry `Connection: close` so idle keep-alive sockets do not hold the process open, and an unref'd timer forces `exit(1)` if draining takes too long. The server timeouts (`connectionTimeout`, `keepAliveTimeout`, `requestTimeout`) bound slow or idle clients.

  ```ts
  await app.listen({ host, port });
  const signal = await new Promise((resolve) => { process.once('SIGINT', resolve); process.once('SIGTERM', resolve); });
  await app.close();
  ```
- **Expected output:** curl prints `{"status":"ok"}`, then `{"waited_ms":1500}` (the slow request completes after the signal), then the last `healthz` fails to connect. The server prints:

  ```
  listening
  received SIGTERM, draining
  stopped
  ```

### 3.5 The Task Tracker service as shipped

- **Goal:** build info, liveness vs readiness, and a drain delay for load balancers.
- **Run:** `ADDR=127.0.0.1:22402 APP_VERSION=1.4.0 GIT_COMMIT=3f9c2ab DRAIN_DELAY_MS=1000 node modules/14-packaging-and-running/examples/05_tracker_service.ts`
- **Code:** [05_tracker_service.ts](examples/05_tracker_service.ts)
- **Walkthrough:** `APP_VERSION` and `GIT_COMMIT` are injected at build time (`docker build --build-arg` + `ENV`, or CI variables) and exposed on `/version`. `/healthz` says "the process is alive" (restart me if this fails); `/readyz` says "route traffic to me" and returns 503 as soon as shutdown starts. `DRAIN_DELAY_MS` keeps serving for a moment after readiness turns red so the balancer can stop routing before the listener closes. The 404 uses the shared error envelope.
- **Expected output:** (`curl -s http://127.0.0.1:22402/<path>`)

  ```
  /healthz  {"status":"ok"}
  /readyz   {"status":"ready"}
  /version  {"name":"task-tracker","version":"1.4.0","commit":"3f9c2ab"}
  /tasks    {"items":[{"id":1,"title":"Write docs","status":"todo","due_date":"2030-01-31","tags":["docs"],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"},{"id":2,"title":"Ship release","status":"in_progress","due_date":null,"tags":[],"owner_id":1,"created_at":"2030-01-02T10:30:00Z"}],"total":2,"limit":20,"offset":0}
  /nope     {"error":{"code":"not_found","message":"route not found"}}   (HTTP 404)
  ```

  After `kill -TERM`, `/readyz` answers `{"status":"draining"}` with status 503 while `/healthz` still answers 200, then the listener closes.

## 4. Common Pitfalls

| Mistake                                                                     | Why it happens                                                                                         | Fix                                                                                                       |
| --------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------- |
| `devDependencies` (typescript, biome, test tools) in the image              | `npm ci` without flags installs everything                                                             | install them only in the build stage; runtime stage gets `npm ci --omit=dev` output                       |
| `CMD npm start` or `CMD ["sh", "-c", ...]`                                  | the shell or npm becomes PID 1 and does not forward `SIGTERM`                                          | `CMD ["node", "dist/main.js"]`                                                                            |
| Container ignores `SIGTERM` and is killed after 10 s                        | Node as PID 1 has no default signal action and no handler registered                                   | register `SIGTERM`/`SIGINT` handlers (or run with `--init`), call `app.close()`                           |
| `Error: Cannot find module .../better_sqlite3.node` or `invalid ELF header` | native module compiled for another libc/arch (e.g. built on macOS or Alpine/musl, run on Debian/glibc) | install dependencies inside the runtime's base image family and architecture, same tag in all stages      |
| `import './x.ts'` fails after `tsc` emit                                    | extensions are not rewritten without the option                                                        | set `rewriteRelativeImportExtensions`, or import `./x.js` consistently                                    |
| Production runs `.ts` with an old Node                                      | type stripping needs Node >= 24; older versions fail with a syntax error                               | `engines` plus a pinned `node:26-slim` tag and a CI matrix; check `process.version` at start-up if needed |
| `node:latest` or `node:slim` as base                                        | the tag moves, builds are not reproducible                                                             | pin a major (or digest) and update deliberately                                                           |
| `COPY . .` before `npm ci`                                                  | any source change invalidates the dependency layer                                                     | copy `package*.json`, install, then copy the sources (exercise 3 checks this)                             |
| `NODE_OPTIONS=--inspect=0.0.0.0:9229` left in production                    | debugging config copied from a dev environment                                                         | audit flags at start-up (exercise 2), never publish the inspector port                                    |
| `NODE_ENV=prod` or `Production`                                             | only the exact string `production` is recognised                                                       | validate `NODE_ENV` with zod in the config layer (module 09)                                              |
| Secrets baked into the image (`COPY .env`, `ARG TOKEN`)                     | build args and layers are readable from the image                                                      | `.dockerignore` for `.env`, inject secrets at run time                                                    |

## 5. Coming From Other Languages

- **JavaScript:** same runtime and npm; the new part is the TypeScript build step (or type stripping) and shipping `dist/`.
- **Java/C#:** `tsc` emit ~ `javac`/`dotnet publish`, `dist/` + `node_modules` ~ a self-contained publish folder; there is no fat jar or single binary by default, the runtime comes from the base image.
- **Python:** `npm ci` against `package-lock.json` ~ `pip install --require-hashes`; `node_modules` is per project (no virtualenv needed). Same PID 1 and `SIGTERM` caveats as with uvicorn/gunicorn.
- **Go:** Go compiles to a static binary and cross-compiles with `GOOS`/`GOARCH`; Node ships source plus a runtime, so images are bigger and native modules are the portability risk. `FROM scratch` is not an option.

## 6. Exercises

Starter code is in [exercises/](exercises/), reference solutions in [solutions/](solutions/). Run every test command from `languages/typescript/`.

| #   | Title                                                                 | Difficulty | Skill practiced                                     | Run tests                                                                                   |
| --- | --------------------------------------------------------------------- | ---------- | --------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| 1   | [Build info](exercises/ex1_build_info/build-info.ts)                  | Easy       | `package.json` + env to build info, `engines` check | `node --test "modules/14-packaging-and-running/exercises/ex1_build_info/**/*.test.ts"`      |
| 2   | [NODE_OPTIONS audit](exercises/ex2_node_options/node-options.ts)      | Medium     | runtime flags, production safety                    | `node --test "modules/14-packaging-and-running/exercises/ex2_node_options/**/*.test.ts"`    |
| 3   | [Dockerfile linter](exercises/ex3_dockerfile_lint/dockerfile-lint.ts) | Stretch    | Dockerfile best practices as code                   | `node --test "modules/14-packaging-and-running/exercises/ex3_dockerfile_lint/**/*.test.ts"` |

**Exercise 1 - Build info.** Task: `parseBuildInfo` (package.json text + env -> name, version, short commit, engine range, with validation), `describeBuild`, and `satisfiesNode` for `>=N[.M[.P]]` ranges. Acceptance: all tests in `build-info.test.ts`. Hint: compare version parts as numbers (`"9" > "10"` is true for strings).

**Exercise 2 - NODE_OPTIONS audit.** Task: tokenize `NODE_OPTIONS` (quotes, escapes), parse it into flags (underscore = dash, last wins) and report unsafe or invalid settings. Acceptance: all tests in `node-options.test.ts`. Hint: write the tokenizer as one loop with two states, "inside quotes" and "outside".

**Exercise 3 - Dockerfile linter.** Task: `lintDockerfile(text)` returns findings with rule ids and line numbers for six rules (unpinned image, root user, shell-form `CMD`, `npm install`, `COPY . .` before install, dev dependencies in the runtime stage). Acceptance: all tests in `dockerfile-lint.test.ts`; the Dockerfile from example 3.3 should produce no findings. Hint: first join `\` continuation lines into instructions and tag each with its stage number, then each rule is a small loop.

## 7. Recap

- Develop with type stripping, ship compiled JavaScript from a multi-stage image with production dependencies only.
- The runtime is configured by flags (`NODE_OPTIONS`) and environment variables, not by code; audit them.
- The process must handle `SIGTERM`: stop accepting, drain, exit 0; liveness and readiness are different questions.
- CI repeats the same commands as local development: `npm ci`, typecheck, lint, test, build.
- When to reach for what: `tsc` emit for services, a pinned slim base image for containers, `--omit=dev` for everything that runs.

## 8. Self-check Questions

1. Why does the runtime stage not run `npm run build` or `npm ci` with dev dependencies?
2. What happens to a `SIGTERM` when the container's `CMD` is `npm start`, and why does it matter during a deployment?
3. Why should `/readyz` fail before the server stops listening, and what does `/healthz` do differently?
4. A native module works in CI but crashes in the container with `invalid ELF header`. List two possible causes.

## 9. Further Reading

- [TypeScript: `rewriteRelativeImportExtensions`](https://www.typescriptlang.org/tsconfig/#rewriteRelativeImportExtensions) and [Node.js: running TypeScript](https://nodejs.org/api/typescript.html)
- [Node.js CLI options and `NODE_OPTIONS`](https://nodejs.org/api/cli.html)
- [Docker: multi-stage builds](https://docs.docker.com/build/building/multi-stage/) and [Dockerfile best practices](https://docs.docker.com/build/building/best-practices/)
- [GitHub Actions: building and testing Node.js](https://docs.github.com/en/actions/use-cases-and-examples/building-and-testing/building-and-testing-nodejs)

---

**Next:** [Language README](../../README.md)
