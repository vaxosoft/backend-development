# Backend Development in TypeScript

Learn backend development with TypeScript 7 on Node.js 24+ and the Fastify framework: language constructs and the core libraries only (HTTP, no gRPC/GraphQL/brokers). Node runs the `.ts` files directly, so there is no build step to learn first. Every lesson uses the same Task Tracker domain described in [../../framework/domain-and-projects.md](../../framework/domain-and-projects.md). Library choices and conventions are in [PROFILE.md](./PROFILE.md).

## Prerequisites

- Programming experience in any language. JavaScript knowledge helps but is not required; module 01 covers the language and its type system.
- [Node.js](https://nodejs.org) 24 or newer (`node --version`) with npm. The repo is built with Node 26; modules 01-12 run on Node 24, but module 13 uses `Temporal`, which needs Node 26.
- A C/C++ toolchain is only needed if `better-sqlite3` has no prebuilt binary for your platform.
- `curl` and an editor with TypeScript support (VS Code).

## Setup

All commands run from this folder (`languages/typescript/`).

```bash
npm ci                                              # installs the pinned dependencies
node modules/01-language-fundamentals/examples/01_types_and_narrowing.ts
```

If npm reports that the `better-sqlite3` install script did not run, run `npm rebuild better-sqlite3`.

| Path                                   | Contents                                                              |
| -------------------------------------- | --------------------------------------------------------------------- |
| `modules/NN-topic/README.md`           | the lesson                                                            |
| `modules/NN-topic/examples/`           | runnable examples: `node modules/02-http-server/examples/01_node_http.ts` |
| `modules/NN-topic/exercises/exN_name/` | starter file plus `*.test.ts` (the tests fail until you solve it)     |
| `modules/NN-topic/solutions/exN_name/` | reference solution files (same names as in the exercise)              |
| `projects/PN-name/`                    | capstone: `README.md`, `starter/`, `tests/`, `solution/`              |

Working on an exercise:

```bash
node --test "modules/02-http-server/exercises/ex1_routes/**/*.test.ts"    # red
# edit the non-test .ts file in that folder until green
```

Peek at `solutions/` only after a real attempt.

Quality commands: `npm run typecheck` (strict `tsc`, no emit), `npm run lint` (Biome check), `npm run format` (Biome fix). Type stripping removes types without checking them, so run `typecheck` regularly.

## Learning path

| #   | Module                                                                                  | Time    |
| --- | --------------------------------------------------------------------------------------- | ------- |
| 01  | [Language fundamentals](./modules/01-language-fundamentals/README.md)                   | 1.5-2 h |
| 02  | [HTTP server](./modules/02-http-server/README.md)                                       | 2 h     |
| 03  | [REST API design](./modules/03-rest-api-design/README.md)                               | 2 h     |
| 04  | [Serialization](./modules/04-serialization/README.md)                                   | 1.5-2h  |
| 05  | [Database access](./modules/05-database-access/README.md)                               | 2h      |
| 06  | [Concurrency](./modules/06-concurrency/README.md)                                       | 2h      |
| 07  | [HTTP client](./modules/07-http-client/README.md)                                       | 1.5-2 h |
| 08  | [File and stream I/O](./modules/08-file-and-stream-io/README.md)                        | 1.5-2 h |
| 09  | [Configuration](./modules/09-configuration/README.md)                                   | 1-1.5 h |
| 10  | [Logging and diagnostics](./modules/10-logging-and-diagnostics/README.md)               | 1.5-2h  |
| 11  | [Auth and security basics](./modules/11-auth-and-security-basics/README.md)             | 2h      |
| 12  | [Testing](./modules/12-testing/README.md)                                               | 2h      |
| 13  | [Time and standard library utilities](./modules/13-time-and-stdlib-utilities/README.md) | 1.5-2h  |
| 14  | [Packaging and running](./modules/14-packaging-and-running/README.md)                   | 1.5-2h  |

Follow the order: modules 02-03 set up the Fastify and error-envelope conventions that later modules reuse, and 06 (the event loop and concurrency) is needed for projects P4 and P6.

## Capstone projects

Each project has a README with requirements and acceptance criteria. Make the tests in `tests/` pass by completing `starter/`.

| Project                                                                           | Exercises modules | Run the tests                                                           |
| --------------------------------------------------------------------------------- | ----------------- | ----------------------------------------------------------------------- |
| [P1 Hello HTTP](./projects/P1-hello-http/README.md)                               | 01, 02, 10        | `node --test "projects/P1-hello-http/tests/**/*.test.ts"`               |
| [P2 Tasks CRUD API](./projects/P2-tasks-crud-api/README.md)                       | 02-05             | `node --test "projects/P2-tasks-crud-api/tests/**/*.test.ts"`           |
| [P3 File service](./projects/P3-file-service/README.md)                           | 02, 08            | `node --test "projects/P3-file-service/tests/**/*.test.ts"`             |
| [P4 API aggregator](./projects/P4-api-aggregator/README.md)                       | 06, 07            | `node --test "projects/P4-api-aggregator/tests/**/*.test.ts"`           |
| [P5 URL shortener](./projects/P5-url-shortener/README.md)                         | 03, 05, 10, 11    | `node --test "projects/P5-url-shortener/tests/**/*.test.ts"`            |
| [P6 Job worker pool](./projects/P6-job-worker-pool/README.md)                     | 06, 10, 13        | `node --test "projects/P6-job-worker-pool/tests/**/*.test.ts"`          |
| [P7 Tested service, Docker, CI](./projects/P7-tested-service-docker-ci/README.md) | 09, 10, 12, 14    | `node --test "projects/P7-tested-service-docker-ci/tests/**/*.test.ts"` |

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

- Only erasable TypeScript syntax is used (no `enum`, `namespace` or constructor parameter properties), because Node strips types rather than compiling them. Relative imports include the `.ts` extension.
- XML is intentionally skipped in module 04 (see [PROFILE.md](./PROFILE.md)).
- Docker is described in module 14 and project P7 but is not needed to run any test.
- PostgreSQL appears only as a compile-only section in module 05 (it connects only when `DATABASE_URL` is set).
- `modules/00-deps-check` only checks the dependency install; ignore it.
- Maintainers: `python3 ../../scripts/verify.py typescript` verifies every exercise (starter fails, solution passes), type checks, and lint.
