# Language Learning Framework

A reusable template for generating consistent backend learning material for any language (Go, Rust, Java, C#, Python, ...).

**Source of truth for scope:** [../backend-core-topics.md](../backend-core-topics.md) (14 topics + 7 projects, HTTP only, no brokers, no advanced distributed concepts).

## Principles
1. **Same outcomes, idiomatic means.** Every language covers the same topics and learning outcomes, but uses its own idioms, not a translation of another language.
2. **Standard library first.** Use the stdlib where reasonable, then the de facto ecosystem library. Justify every third-party dependency in the language profile.
3. **Runnable and tested.** Every example compiles/runs, and every exercise has tests that fail until solved.
4. **Small and focused.** One concept per example; a module takes roughly 1-2 hours.
5. **Pitfalls are first-class.** Each module lists common mistakes for that language.
6. **Cumulative.** Later modules may reuse earlier ones; capstone projects combine them.

## Repository Layout

```
web-development/
├── backend-core-topics.md          # scope (language-agnostic)
├── framework/                      # this template system
│   ├── README.md
│   ├── language-profile.template.md
│   ├── module.template.md
│   ├── generation-prompt.md
│   └── domain-and-projects.md      # shared Task Tracker domain, wire format, capstone specs
├── scripts/verify.py               # verifies a language track (see Repository Mechanics)
└── languages/
    └── <lang>/                     # e.g. go, rust, java
        ├── PROFILE.md              # filled from language-profile.template.md
        ├── README.md               # learning path + progress checklist
        ├── modules/
        │   └── NN-<topic>/         # NN and topic names match backend-core-topics.md
        │       ├── README.md       # lesson, from module.template.md
        │       ├── examples/       # runnable, one concept each
        │       ├── exercises/      # starter code + failing tests
        │       └── solutions/      # reference solutions
        └── projects/
            └── PN-<name>/          # capstones (README, starter, tests)
```

Module folder names (fixed across languages):

| NN | Folder |
|----|--------|
| 01 | `01-language-fundamentals` |
| 02 | `02-http-server` |
| 03 | `03-rest-api-design` |
| 04 | `04-serialization` |
| 05 | `05-database-access` |
| 06 | `06-concurrency` |
| 07 | `07-http-client` |
| 08 | `08-file-and-stream-io` |
| 09 | `09-configuration` |
| 10 | `10-logging-and-diagnostics` |
| 11 | `11-auth-and-security-basics` |
| 12 | `12-testing` |
| 13 | `13-time-and-stdlib-utilities` |
| 14 | `14-packaging-and-running` |

## Workflow for Adding a Language
1. Copy `language-profile.template.md` to `languages/<lang>/PROFILE.md` and fill it in (toolchain, library choices per topic, conventions).
2. Review the profile (this is the control point; everything generated depends on it).
3. For each module, run `generation-prompt.md` with the profile and topic as inputs. Output follows `module.template.md`.
4. Verify: examples run, exercise tests fail on starter and pass on solution, lint/format clean.
5. Generate `languages/<lang>/README.md` (learning path, prerequisites, progress checklist).
6. Generate the capstone projects.

## Definition of Done (per module)
- [ ] All sections of `module.template.md` present
- [ ] Every example builds and runs with documented commands
- [ ] Every exercise has tests; starter fails, solution passes
- [ ] Idiomatic per the language profile (formatter and linter clean)
- [ ] Pitfalls section is language-specific, not generic
- [ ] No concepts outside scope (no gRPC/GraphQL, brokers, distributed algorithms)
- [ ] Dependencies pinned and listed in the profile

## Repository Mechanics (contract for every language track)

Exercises, solutions and projects follow one mechanical rule so a single script can verify them:

- `exercises/` holds **starter code + tests**. Starter code compiles but fails the tests (`todo!()`, `throw new NotImplementedException()`, `panic("TODO")`, or returning zero values).
- `solutions/` holds **only the files that differ**, at the same relative paths. Overlaying `solutions/` onto `exercises/` must make every test pass. Test files are never in `solutions/`.
- Projects: `projects/PN-name/{README.md, starter/, solution/, tests/}`. `solution/` overlays `starter/`; `tests/` is never overlaid. (Rust keeps its tests in `starter/tests/`.)
- Third-party dependencies are pinned once per language (see each `PROFILE.md`). Do not add dependencies while writing a module; if one is truly needed, update the profile first.
- **Go exception:** Go folders are named `_solutions/` and `_solution/` (leading underscore) because the Go toolchain ignores such directories; otherwise partial overlays would break `go build ./...` and `go vet ./...`.
- Build artifacts (`bin/`, `obj/`, `target/`) are git-ignored.

Verify a whole track (works on a temp copy, never edits the repo):

```bash
scripts/verify.py go        # or: rust | csharp
scripts/verify.py go 05-    # only units whose name contains "05-"
```

The script checks: starter tests fail on assertions (not compile errors) -> solution passes -> formatter, linter, and every example builds.

Capstone databases: tests run on SQLite so they work anywhere. PostgreSQL is taught in module 05 as a driver/connection-string swap and is not exercised by the automated tests.

## Standard Running Domain
To keep context consistent across modules and languages, all examples use the same small domain: a **Task Tracker** (users, tasks with title/status/due date/tags). The exact fields, wire format, error envelope and capstone specs are in [domain-and-projects.md](./domain-and-projects.md). It is simple enough for CRUD, auth, concurrency, and file I/O examples.
