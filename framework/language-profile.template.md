# Language Profile: <Language>

Fill this in once per language before generating any module. It constrains all generated content.

## 1. Overview
- Language / version targeted:
- Paradigm and typing (static/dynamic, GC/ownership, OOP/functional/mixed):
- Why it is used for backend services (2-3 sentences):

## 2. Toolchain

| Purpose | Tool | Command |
|---------|------|---------|
| Install / version manager | | |
| Package / dependency manager | | |
| Build | | |
| Run | | |
| Test | | |
| Format | | |
| Lint / static analysis | | |
| Debug / profile | | |

## 3. Project Conventions
- Standard project layout:
- Naming and visibility conventions:
- Error handling style (the idiom to teach):
- Documentation comment style:

## 4. Library Choices per Topic
Prefer stdlib; justify third-party libraries.

| Module | Stdlib / Library | Version | Why |
|--------|------------------|---------|-----|
| 02 HTTP server | | | |
| 03 REST design (routing/validation) | | | |
| 04 Serialization | | | |
| 05 Database access (driver/ORM/query builder) | | | |
| 06 Concurrency | | | |
| 07 HTTP client | | | |
| 08 File and stream I/O | | | |
| 09 Configuration | | | |
| 10 Logging | | | |
| 11 Auth / security | | | |
| 12 Testing / mocking | | | |
| 13 Time and utilities | | | |
| 14 Packaging | | | |

Database used for examples (default: SQLite for portability, PostgreSQL for the capstone):

## 5. Language-specific Teaching Notes
Concepts that need extra emphasis for this language (e.g. ownership/lifetimes, goroutine leaks, nullability, async coloring, checked exceptions).

| Module | Emphasis | Typical pitfall |
|--------|----------|-----------------|
| 01 | | |
| 06 | | |
| ... | | |

## 6. Comparison Anchors
Languages learners may already know, used for "coming from X" callouts:
-

## 7. Module Ordering Adjustments
Default order is 01-14. Note any reordering or prerequisites specific to this language (e.g. introduce error handling before HTTP).

## 8. Out of Scope for This Language
Features intentionally skipped (e.g. macros, reflection internals, unsafe code).
