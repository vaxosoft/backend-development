# Web Development: Backend Learning Paths

A repo for learning backend development by language. It covers language constructs and core libraries only: HTTP services, no gRPC, GraphQL, message brokers or advanced distributed-systems topics. All tracks use the same Task Tracker domain, so you can compare how each language solves the same problem.

| Track | Stack | Start here |
|-------|-------|------------|
| Go | Go 1.25+, `net/http`, `database/sql`, `log/slog` | [languages/go](./languages/go/README.md) |
| Rust | Rust 2024, tokio, axum, sqlx, serde | [languages/rust](./languages/rust/README.md) |
| C# | C# 14, .NET 10, ASP.NET Core minimal APIs, ADO.NET/EF Core | [languages/csharp](./languages/csharp/README.md) |
| TypeScript | TypeScript 7, Node 24+, Fastify, zod, `node:test` (no build step) | [languages/typescript](./languages/typescript/README.md) |
| Python | Python 3.14, FastAPI, pydantic, `sqlite3`, pytest, mypy strict | [languages/python](./languages/python/README.md) |

Each track has 14 modules (lesson, runnable examples, three exercises with tests and solutions) and 7 capstone projects (starter, tests, reference solution).

- [backend-core-topics.md](./backend-core-topics.md): the language-agnostic scope (14 topics, 7 projects)
- [framework/](./framework/README.md): templates, conventions, shared domain and project specs. Use it to add another language
- [site/](./site/README.md): a navigable web UI (search, language switcher, compare pages, progress tracking, code viewer) generated from this repo; deploys to GitHub Pages with the included workflow
- `scripts/verify.py <go|rust|csharp|typescript|python>`: checks that every exercise and project starter fails its tests, that the solution passes, and that format, lint and builds are clean
