# Backend Development Core Topics

Language-agnostic list of topics for learning a language and its core backend libraries/frameworks (Go, Rust, Java, C#, etc.). Scope: HTTP-based backend services only. No gRPC/GraphQL, message brokers, or advanced distributed-systems concepts.

Each topic can become a learning module: concept, idiomatic code sample, common pitfalls, exercises.

## 1. Language Fundamentals
- Types, variables, constants, control flow, functions
- Structs/classes/records, enums, interfaces/traits, generics
- Composition vs inheritance, polymorphism
- Error handling idioms (errors/exceptions/Result), custom errors
- Memory management model (GC vs ownership), pointers/references, value vs reference semantics
- Collections, strings, iteration, closures
- Modules/packages, visibility, project layout
- Tooling: package manager, build, formatter, linter, debugger

## 2. HTTP Server
- Routing, path/query params, handlers, middleware
- Request parsing: headers, body, forms, multipart, file uploads
- Response writing: status codes, headers, streaming
- Content negotiation, CORS, cookies
- Static files, graceful shutdown, server timeouts
- Framework conventions (e.g. net/http + chi/gin, axum/actix, Spring, ASP.NET)

## 3. REST API Design
- Resource modeling, HTTP verbs, status codes
- Request validation, error response format
- Pagination, filtering, sorting
- Versioning, idempotency, OpenAPI documentation

## 4. Serialization
- JSON encode/decode, struct tags/attributes, custom (de)serializers
- Optional/null handling, unknown fields, date/time formats
- Other formats: XML, CSV, YAML, form-encoded (brief)
- Validation of decoded data

## 5. Database Access
- SQL basics from code: connections, pooling, queries, prepared statements
- Mapping rows to types; null handling; query builders vs ORMs
- Transactions and isolation levels
- Migrations, repository pattern, testing with a real DB
- Intro to one NoSQL/key-value store from code (e.g. Redis) as optional

## 6. Concurrency
- Threads, async/await, goroutines/tasks, event loop model
- Channels, mutexes, atomics, shared state vs message passing
- Worker pools, fan-out/fan-in, pipelines
- Cancellation, timeouts, context propagation
- Race conditions, deadlocks, race detectors

## 7. HTTP Client
- Making requests, headers, bodies, JSON round-trips
- Timeouts, retries with backoff, connection reuse
- Handling errors and non-2xx responses
- Calling third-party APIs, authentication headers
- Mocking/testing HTTP clients

## 8. File and Stream I/O
- Reading/writing files, buffering, streaming large data
- Paths, directories, temp files
- Readers/writers/streams abstractions
- Compression, encoding (base64, hex), hashing
- Embedding assets

## 9. Configuration and Environment
- Environment variables, config files, flags
- Layered configuration, defaults, validation
- Secrets handling basics
- Dependency injection / wiring (manual vs framework)

## 10. Logging and Diagnostics
- Structured logging, log levels, contextual fields
- Request logging middleware, correlation IDs
- Panic/exception recovery
- Basic metrics and health endpoints
- Profiling and debugging basics

## 11. Authentication and Security Basics
- Password hashing, sessions/cookies, JWT, API keys
- Auth middleware, role-based access
- Input validation, SQL injection, XSS, CSRF basics
- TLS/HTTPS basics, rate limiting

## 12. Testing
- Unit tests, table-driven tests, assertions
- HTTP handler tests (test server/recorder)
- Mocks/fakes, dependency isolation
- Integration tests with DB (containers or in-memory)
- Benchmarks, coverage

## 13. Time, Utilities and Standard Library
- Dates, time zones, durations, timers/tickers
- UUIDs, random numbers, regex, text templating
- Hashing/crypto primitives
- Scheduling background jobs in-process

## 14. Packaging and Running
- Building binaries/artifacts, cross-compilation
- Dockerfile for the service
- Basic CI pipeline (build, test, lint)
- Runtime flags and production settings

## Suggested Hands-on Projects (per language)
1. Hello HTTP server with routing, middleware, and graceful shutdown
2. JSON CRUD REST API backed by SQL database
3. File upload/download service with streaming
4. Concurrent web scraper / API aggregator using the HTTP client
5. URL shortener with auth, rate limiting, and structured logging
6. Background worker pool processing jobs inside the service
7. Fully tested service with Dockerfile and CI
