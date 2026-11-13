# Language Profile: TypeScript

Constrains all generated TypeScript content. Dependencies are pinned in [package.json](./package.json) (exact versions) and `package-lock.json`. Do not add dependencies while writing a module.

## 1. Overview

- **Version:** TypeScript 7.0 on Node.js 24+ (developed on Node 26; module 13 requires Node 26 for the built-in `Temporal`). ES modules only (`"type": "module"`).
- **Paradigm / typing:** statically typed superset of JavaScript, structural typing, garbage collected, single-threaded event loop with `async`/`await`. Functional-leaning with classes available. Types are erased at runtime.
- **No build step in this repo.** Node runs `.ts` files directly by stripping types ("type stripping"). `tsc` is used only to type-check (`noEmit`). Module 14 shows the emit/build path for production.
- **Why backend:** one language for the whole stack, a huge ecosystem, excellent I/O concurrency, fast iteration. Types catch API-shape mistakes that plain JavaScript leaves to runtime. Types do not validate untrusted input, so the lessons pair them with runtime validation (zod).

## 2. Toolchain

| Purpose                      | Tool                                                                                     | Command                                                                  |
| ---------------------------- | ---------------------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| Install / version manager    | Node.js (nvm, fnm, or official installer)                                                | `node --version` (24 or newer)                                           |
| Package / dependency manager | npm                                                                                      | `npm ci`                                                                 |
| Build                        | none for lessons (type stripping); `tsc` for production builds in module 14              | `node file.ts`                                                           |
| Run an example               | Node                                                                                     | `node modules/02-http-server/examples/01_hello.ts`                       |
| Test                         | `node:test` (built in)                                                                   | `node --test "modules/02-http-server/exercises/ex1_routes/**/*.test.ts"` |
| Type check                   | `tsc`                                                                                    | `npm run typecheck`                                                      |
| Format + lint                | Biome                                                                                    | `npm run lint` (check) / `npm run format` (fix)                          |
| Debug / profile              | `node --inspect`, `node --watch`, `--cpu-prof`, `node:test --experimental-test-coverage` |                                                                          |

All commands run from `languages/typescript/`. If `npm ci` reports that the `better-sqlite3` install script was not run, run `npm rebuild better-sqlite3`.

## 3. Project Conventions

- **Layout of this repo:** one npm package at `languages/typescript/`, one `tsconfig.json`, one `biome.json`.
  - Examples: `modules/NN-topic/examples/NN_name.ts`, run with `node <path>`. A helper shared by a module's examples is `examples/_shared.ts` (not runnable).
  - Exercises: `modules/NN-topic/exercises/exN_name/{name.ts, name.test.ts}`. Starter code compiles and its functions `throw new Error('TODO')`, so tests fail at runtime on assertions. Test files import the code with `./name.ts`.
  - Solutions: `modules/NN-topic/solutions/exN_name/name.ts` (only non-test files; same relative path as in the exercise). `tsconfig.json` excludes `solutions/` and `solution/`.
  - Projects: `projects/PN-name/{README.md, starter/, solution/, tests/}`. `starter/src/app.ts` exports `buildApp(config)`, `starter/src/main.ts` is the runnable entry. Tests are `projects/PN-name/tests/*.test.ts` importing `../starter/src/app.ts`.
  - `modules/00-deps-check` is infrastructure; do not edit.
- **Imports:** relative imports always include the `.ts` extension (`import { x } from './x.ts'`). Use `import type` for type-only imports (`verbatimModuleSyntax`). Built-ins use the `node:` prefix.
- **Erasable syntax only** (`erasableSyntaxOnly`): no `enum`, no `namespace`, no constructor parameter properties. Teach string-literal unions and `as const` objects instead.
- **Naming:** `camelCase` values and functions, `PascalCase` types and classes, `SCREAMING_SNAKE_CASE` only for true constants, `kebab-case.ts` file names, `.test.ts` suffix for tests.
- **Error handling style:** throw `Error` subclasses (with `cause`) for exceptional failures; return discriminated unions (`{ ok: true, value } | { ok: false, error }`) or `null`/`undefined` for expected outcomes; validate untrusted input with zod `safeParse`; never `catch` and ignore; `unknown` in `catch` clauses and narrow with `instanceof`. HTTP errors become the shared envelope through a Fastify `setErrorHandler`.
- **Types:** `strict` plus `noUncheckedIndexedAccess`. No `any` (Biome flags it). Prefer `interface` for object shapes that are extended and `type` for unions. Use `satisfies` over casts. Avoid non-null `!`.
- **Docs:** TSDoc `/** ... */` on exported items, only where it adds information.

## 4. Library Choices per Topic

Node built-ins first. Third-party packages are listed with the reason.

| Module                 | Library                                                                                                                                                                             | Version                          | Why                                                                                                                                         |
| ---------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| 01 Fundamentals        | TypeScript language, built-ins (`Map`, `Set`, iterators, generators), `node:assert`                                                                                                 | TS 7.0.2                         |                                                                                                                                             |
| 02 HTTP server         | `node:http` (first example), `fastify` + `@fastify/static`, `@fastify/multipart`, `@fastify/cors`, `@fastify/cookie`                                                                | fastify 5.12.8, plugins 10-11.x  | Node has no router. Fastify is small, fast, plugin-based, has built-in `inject()` for tests                                                 |
| 03 REST design         | Fastify routing and hooks, `zod` for validation, `fastify-plugin`                                                                                                                   | zod 4.6.7                        | one runtime schema also yields the static type. Fastify JSON-schema validation is mentioned, not used. OpenAPI generation is mentioned only |
| 04 Serialization       | `JSON`, `zod`, `csv-parse` / `csv-stringify`, `yaml`, `URLSearchParams`                                                                                                             | csv 7.0.3, yaml 2.9.1            | CSV/YAML are not built in. XML is skipped (no stable built-in) and noted in the lesson                                                      |
| 05 Database access     | `better-sqlite3` (examples; synchronous API), `pg` (compile-only section, runs only if `DATABASE_URL` is set)                                                                       | better-sqlite3 13.1.1, pg 8.23.1 | SQLite needs no server. `better-sqlite3` is the standard native driver. `pg` is the standard PostgreSQL driver. No ORM                      |
| 06 Concurrency         | `async`/`await`, `Promise.all/allSettled/race/any`, `AbortController`, `node:worker_threads`, `node:events`, `node:timers/promises`, streams                                        | built in                         |                                                                                                                                             |
| 07 HTTP client         | global `fetch` (undici), `AbortSignal.timeout`, hand-written retry/backoff; tests use a local `node:http` server                                                                    | built in                         | no axios/got needed                                                                                                                         |
| 08 File and stream I/O | `node:fs/promises`, `node:stream/promises`, `node:path`, `node:os`, `node:zlib`, `node:crypto`, `Buffer` (base64/hex), `node:readline`                                              | built in                         |                                                                                                                                             |
| 09 Configuration       | `process.env`, `node:util` `parseArgs`, `process.loadEnvFile`, `zod`, `JSON.parse` of a file                                                                                        | built in + zod                   | layering is hand-written to show the pattern; no dotenv/convict                                                                             |
| 10 Logging             | `pino` (Fastify's native logger), Fastify hooks for request logging, `node:async_hooks` `AsyncLocalStorage`, `node:perf_hooks`, `process.on('uncaughtException')`, `node --inspect` | pino 10.4.0                      | pino is the de facto structured logger                                                                                                      |
| 11 Auth / security     | `node:crypto` (`scrypt`, `randomBytes`, `timingSafeEqual`, `createHmac`), `jose` (JWT), `@fastify/cookie`, hand-written token bucket, `node:https`                                  | jose 6.2.17                      | scrypt is built in (no native bcrypt/argon2 needed); jose is the standard dependency-free JWT library                                       |
| 12 Testing             | `node:test` (`test`, `describe`, `mock`, `mock.timers`, coverage), `node:assert/strict`, Fastify `inject()`                                                                         | built in                         | no jest/vitest/sinon                                                                                                                        |
| 13 Time and utilities  | `Temporal` (built into Node 26), `Date`, `Intl.DateTimeFormat`, `crypto.randomUUID`, `crypto.randomInt`, `RegExp`, template literals, `node:timers/promises`                        | built in                         |                                                                                                                                             |
| 14 Packaging           | `tsc` emit with `rewriteRelativeImportExtensions`, `npm ci --omit=dev`, multi-stage Dockerfile on `node:26-slim`, GitHub Actions                                                    |                                  |                                                                                                                                             |

**Database for examples:** SQLite via `better-sqlite3` (`:memory:` or a temp file). PostgreSQL is a documented swap: the example imports `pg` and only connects when `DATABASE_URL` is set, otherwise it prints a skip message.

## 5. Language-specific Teaching Notes

| Module | Emphasis                                                                                                                                                                | Typical pitfall                                                                                                                                                     |
| ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 01     | structural typing, unions and narrowing, `unknown` vs `any`, generics, `readonly`, `Map`/`Set`, iterators, closures, `this`, ESM imports, errors with `cause`           | `==` vs `===`, `any` leaking, mutating shared objects, `Array.sort` default (string) order and in-place mutation, `typeof null`                                     |
| 02     | request/reply lifecycle, hooks, plugins and encapsulation, decorators on the Fastify instance, `listen`, graceful `close()`                                             | forgetting to `return reply`/await in async handlers, sending a reply twice, `listen` host `localhost` vs `127.0.0.1`, unhandled rejection in a handler             |
| 03     | shared error envelope via `setErrorHandler`, validation with zod, status codes, pagination clamp                                                                        | trusting TypeScript types for request bodies (types are erased), returning `200` with an error body, `Number(undefined)` is `NaN`                                   |
| 04     | `JSON.stringify` quirks (`undefined`, `Date`, `bigint`, `Map`), `zod` transforms and strict objects, nullable vs optional, date handling                                | `JSON.parse` returns `any`, `bigint` throws in `JSON.stringify`, numbers lose precision above 2^53                                                                  |
| 05     | prepared statements, transactions (`db.transaction`), synchronous driver blocks the event loop, repository pattern                                                      | string-concatenated SQL, forgetting to close the DB, long synchronous queries blocking all requests, `:memory:` is per connection                                   |
| 06     | event loop phases, microtasks vs macrotasks, `Promise` combinators, concurrency limits by hand, `AbortSignal`, `worker_threads` for CPU work                            | `await` in a loop serialising work, unhandled promise rejections, forgotten `await`, CPU-bound code blocking the loop, `Promise.all` fail-fast leaving work running |
| 07     | `fetch` does not reject on non-2xx, `AbortSignal.timeout`, consuming/cancelling bodies, retry only idempotent requests                                                  | not checking `res.ok`, never reading the body, no explicit timeout (undici's defaults only cut off after 5 minutes), retrying non-idempotent requests               |
| 08     | streams and backpressure, `pipeline`, `fs/promises`, `Buffer`, path traversal, `crypto.createHash` on a stream                                                          | reading a huge file with `readFile`, ignoring stream errors, `path.join` with user input, `fs.exists` race (use try/catch)                                          |
| 09     | typed config object from validated env, precedence defaults < file < env < CLI flags, fail fast                                                                         | env values are always strings (`"false"` is truthy), reading `process.env` everywhere, secrets in logs                                                              |
| 10     | pino levels and child loggers, request-id via `AsyncLocalStorage`, redaction, `unhandledRejection` policy                                                               | logging secrets/PII, `console.log` in production, swallowing errors in hooks                                                                                        |
| 11     | scrypt with random salt, `timingSafeEqual` (equal-length buffers required), JWT verification options (`algorithms`, `exp`), cookie flags, parameterised SQL, rate limit | `===` on secrets, accepting `alg: none` or unpinned algorithms, storing tokens in `localStorage` in the browser, forgetting to verify `exp`                         |
| 12     | `node:test` structure, `mock.fn`/`mock.method`, fake timers, `inject()` without a network, isolating state per test                                                     | tests sharing module-level state, unawaited async assertions (test passes anyway), leaving servers open so the process hangs                                        |
| 13     | `Temporal` (`Instant`, `ZonedDateTime`, `PlainDate`, `Duration`) vs legacy `Date`, `Intl`, timers and `unref()`, regex named groups                                     | `Date` month is 0-based, `new Date('2030-01-01')` is UTC but `new Date('2030-01-01T00:00')` is local, `Math.random` for secrets, `setInterval` drift                |
| 14     | `tsc` build vs type stripping, `engines`, `npm ci --omit=dev`, non-root `USER node`, layer caching, native module `better-sqlite3` in slim images                       | shipping `devDependencies`, running `.ts` in production without pinning Node >= 24, native module built for a different libc/arch than the runtime image            |

## 6. Comparison Anchors

JavaScript (this is the superset), Java/C# (static types, classes, `async`/`await` in C#), Python (dynamic runtime vs erased types), Go (event loop vs goroutines).

## 7. Module Ordering Adjustments

Default order 01-14. Module 01 includes ESM, async basics (promises) and error handling before HTTP. Module 06 deepens the event loop and should be read before the concurrency-heavy projects P4 and P6.

## 8. Out of Scope for This Language

Frontend/React, other server frameworks (Express, NestJS, Koa, Hono), decorators and class-heavy DI containers, ORMs (Prisma, TypeORM, Drizzle), bundlers (webpack, esbuild, Vite), `ts-node`/`tsx`, Jest/Vitest, GraphQL, gRPC, WebAssembly, native addons (writing them), advanced type-level programming (conditional/mapped types beyond a mention).
