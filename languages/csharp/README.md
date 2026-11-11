# Backend Development in C#

Learn backend development with C# 14 on .NET 10 and ASP.NET Core: language constructs and the core libraries only (HTTP, no gRPC/GraphQL/brokers). Every lesson uses the same Task Tracker domain described in [../../framework/domain-and-projects.md](../../framework/domain-and-projects.md). Library choices and conventions are in [PROFILE.md](./PROFILE.md).

## Prerequisites
- Programming experience in any language (Java, TypeScript, Python, ...). No prior C# needed; module 01 covers the language.
- [.NET 10 SDK](https://dotnet.microsoft.com/download) (`dotnet --version` prints 10.x). Nothing else: the database is SQLite, there are no containers to run.
- `curl` and an editor (VS Code with C# Dev Kit, or Visual Studio / Rider).

## Setup
All commands run from this folder (`languages/csharp/`).

```bash
dotnet build modules/01-language-fundamentals/examples   # restores packages (needs internet once)
dotnet run --project modules/01-language-fundamentals/examples -- 01-values   # running with no name prints the list of examples
```

How the repo is organised (details in [PROFILE.md](./PROFILE.md) section 3):

| Path | Contents |
|------|----------|
| `modules/NN-topic/README.md` | the lesson |
| `modules/NN-topic/examples/` | one console/web project; run an example with `dotnet run --project <dir> -- <example-name>` |
| `modules/NN-topic/exercises/` | xUnit project with starter code and tests (they fail until you solve them) |
| `modules/NN-topic/solutions/` | reference solutions; same file names as in `exercises/` |
| `projects/PN-name/` | capstone: `README.md`, `starter/`, `tests/`, `solution/` |

Working on an exercise:

```bash
dotnet test modules/02-http-server/exercises --filter "FullyQualifiedName~.Ex1_"   # red
# edit modules/02-http-server/exercises/Ex1_*.cs until green
```
To peek at a solution, open the file with the same name in `solutions/`.

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
| 09 | [Configuration](./modules/09-configuration/README.md) | 1.5h |
| 10 | [Logging and diagnostics](./modules/10-logging-and-diagnostics/README.md) | 1.5-2h |
| 11 | [Auth and security basics](./modules/11-auth-and-security-basics/README.md) | 2h |
| 12 | [Testing](./modules/12-testing/README.md) | 2h |
| 13 | [Time and standard library utilities](./modules/13-time-and-stdlib-utilities/README.md) | 1.5-2h |
| 14 | [Packaging and running](./modules/14-packaging-and-running/README.md) | 1.5-2h |

Follow the order: DI (02), ProblemDetails (03) and async (06) are reused later. Modules 09-10 can be swapped.

## Capstone projects
Each project has a README with requirements and acceptance criteria. Make the tests in `tests/` pass by completing `starter/`.

| Project | Exercises modules | Run the tests |
|---------|-------------------|---------------|
| [P1 Hello HTTP](./projects/P1-hello-http/README.md) | 01, 02, 10 | `dotnet test projects/P1-hello-http/tests` |
| [P2 Tasks CRUD API](./projects/P2-tasks-crud-api/README.md) | 02-05 | `dotnet test projects/P2-tasks-crud-api/tests` |
| [P3 File service](./projects/P3-file-service/README.md) | 02, 08 | `dotnet test projects/P3-file-service/tests` |
| [P4 API aggregator](./projects/P4-api-aggregator/README.md) | 06, 07 | `dotnet test projects/P4-api-aggregator/tests` |
| [P5 URL shortener](./projects/P5-url-shortener/README.md) | 03, 05, 10, 11 | `dotnet test projects/P5-url-shortener/tests` |
| [P6 Job worker pool](./projects/P6-job-worker-pool/README.md) | 06, 10, 13 | `dotnet test projects/P6-job-worker-pool/tests` |
| [P7 Tested service, Docker, CI](./projects/P7-tested-service-docker-ci/README.md) | 09, 10, 12, 14 | `dotnet test projects/P7-tested-service-docker-ci/tests` |

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
- Warnings are errors (`TreatWarningsAsErrors`); fix them rather than suppressing.
- Docker is described in module 14 and project P7 but is not needed to run any test.
- PostgreSQL appears only as a compile-only section in module 05.
- Maintainers: `python3 ../../scripts/verify.py csharp` verifies every exercise (starter fails, solution passes), formatting, and builds.
