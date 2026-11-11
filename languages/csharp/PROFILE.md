# Language Profile: C#

Constrains all generated C# content. Package versions are pinned centrally in [Directory.Packages.props](./Directory.Packages.props) and the compiler settings live in [Directory.Build.props](./Directory.Build.props). Project files use `<PackageReference Include="X" />` without a version. Do not add packages while writing a module.

## 1. Overview
- **Version:** C# 14 on .NET 10 (LTS), ASP.NET Core 10. `net10.0`, nullable reference types on, implicit usings on, warnings are errors.
- **Paradigm / typing:** statically typed, garbage collected, managed runtime (CLR). Object-oriented with strong functional additions (records, pattern matching, LINQ, `async`/`await`). Value types (`struct`) and reference types (`class`).
- **Why backend:** ASP.NET Core is one of the fastest mainstream web stacks, `async`/`await` originated here, and the platform ships dependency injection, configuration, logging, and hosting in the box.

## 2. Toolchain

| Purpose | Tool | Command |
|---------|------|---------|
| Install / version manager | .NET SDK installer (`dotnet-install.sh` or package manager) | `dotnet --list-sdks` |
| Package / dependency manager | NuGet via `dotnet` CLI, central package management | `dotnet add package <Id>` (then move the version to `Directory.Packages.props`) |
| Build | `dotnet build` | `dotnet build modules/02-http-server/examples` |
| Run an example | `dotnet run` with the example name as argument | `dotnet run --project modules/02-http-server/examples -- 01-hello` |
| Test | `dotnet test` (xUnit) | `dotnet test modules/02-http-server/exercises --filter "FullyQualifiedName~.Ex1_"` |
| Format | `dotnet format` | `dotnet format whitespace <project>` |
| Lint / static analysis | Roslyn analyzers (built in, warnings as errors) | `dotnet build` |
| Debug / profile | VS Code C# Dev Kit / Visual Studio, `dotnet-counters`, `dotnet-trace` | |

All commands run from `languages/csharp/`. This repo was built against SDK 10.0.404.

## 3. Project Conventions
- **Layout of this repo:** there is no solution-wide project; each module has two sibling projects (never nested, because MSBuild globs `**/*.cs` below a project folder):
  - `modules/NN-topic/examples/MNN.Examples.csproj`: one console (or web) app containing many examples. `Program.cs` is a tiny dispatcher: `dotnet run --project ... -- <example-name>`. Each example is a file `NN_name.cs` with a `static class` exposing `public static Task<int> RunAsync(string[] args)`.
  - `modules/NN-topic/exercises/MNN.Exercises.csproj`: xUnit project holding starter code and its tests. Files are `ExN_Name.cs` (starter implementation, class `ExN_Name`... ) and `ExN_NameTests.cs` (tests). Every test class name starts with `ExN_` so `--filter "FullyQualifiedName~.ExN_"` selects one exercise. Starter code throws `NotImplementedException`.
  - `modules/NN-topic/solutions/ExN_Name.cs`: replaces the same-named file in `exercises/`. Tests are never in `solutions/`.
  - `projects/PN-name/starter/*.csproj` (+`solution/` overlay) and `projects/PN-name/tests/*.csproj` (xUnit, references `../starter`).
  - `modules/00-deps-check` is infrastructure; do not edit.
- **Test project template:** use `modules/00-deps-check/tests/M00.DepsCheck.Tests.csproj` as the model (note `<Using Include="Xunit" />`). Web examples use `Microsoft.NET.Sdk.Web`. Console examples that need hosting/logging add `<FrameworkReference Include="Microsoft.AspNetCore.App" />` or use the Web SDK instead of extra packages.
- **Naming:** `PascalCase` for types/methods/properties, `camelCase` for locals/parameters, `_camelCase` for private fields, `I` prefix for interfaces, `Async` suffix for async methods, file-scoped namespaces.
- **Error handling:** exceptions for exceptional cases (custom exception types deriving from `Exception`), result-style returns (`TryParse` pattern, `Results.NotFound()`, a small `Result<T>` record) for expected failures; `ProblemDetails` (RFC 9457) as the HTTP error body; never `catch (Exception)` and swallow.
- **Docs:** `///` XML doc comments on public APIs.
- **Style rules:** records for DTOs, `required` members, primary constructors, `using` declarations, `CancellationToken` as the last parameter of async APIs.

## 4. Library Choices per Topic

| Module | Library | Version | Why |
|--------|---------|---------|-----|
| 01 Fundamentals | BCL (`System.Collections.Generic`, LINQ, records, pattern matching) | .NET 10 | |
| 02 HTTP server | ASP.NET Core Minimal APIs (`WebApplication`, middleware, `app.MapGet`) | in the shared framework | the modern default; controllers are mentioned but not taught |
| 03 REST design | Minimal API built-in validation (`AddValidation`, DataAnnotations), `ProblemDetails`, `Microsoft.AspNetCore.OpenApi` | 10.0.15 | OpenAPI doc generation is in-box in .NET 9+ |
| 04 Serialization | `System.Text.Json` (+ source generators), `CsvHelper`, `YamlDotNet`, `System.Xml.Serialization` | CsvHelper 34.0.0, YamlDotNet 18.1.0 | STJ is the default; CSV/YAML not in the BCL |
| 05 Database access | ADO.NET with `Microsoft.Data.Sqlite`; one EF Core 10 example (`Microsoft.EntityFrameworkCore.Sqlite`); `Npgsql` compile-only section | 10.0.15 / Npgsql 10.0.3 | ADO.NET shows the fundamentals, EF Core is what most teams use |
| 06 Concurrency | `Task`, `async`/`await`, `System.Threading.Channels`, `SemaphoreSlim`, `Parallel.ForEachAsync`, `CancellationToken`, `lock`, `Interlocked`, `ConcurrentDictionary` | BCL | |
| 07 HTTP client | `HttpClient` + `IHttpClientFactory`, `Microsoft.Extensions.Http.Resilience` (Polly v8 based) for retry/timeout, `WebApplicationFactory`/`HttpMessageHandler` fake for tests | 10.11.0 | factory avoids socket exhaustion; resilience is the in-box retry story |
| 08 File and stream I/O | `System.IO` (`File`, `Path`, `Stream`, `StreamReader`), `System.IO.Compression`, `Convert`, `System.Security.Cryptography`, embedded resources | BCL | |
| 09 Configuration | `IConfiguration` layering (appsettings.json, environment, command line), Options pattern (`IOptions<T>`, validation), user-secrets, DI | in the shared framework | the built-in system is the idiom |
| 10 Logging | `Microsoft.Extensions.Logging` (`ILogger<T>`, scopes, JSON console formatter), `Microsoft.Extensions.Diagnostics.HealthChecks`, `System.Diagnostics.Metrics`, exception-handling middleware | 10.0.15 | |
| 11 Auth / security | `Rfc2898DeriveBytes.Pbkdf2`, `CryptographicOperations.FixedTimeEquals`, `Microsoft.AspNetCore.Authentication.JwtBearer`, `Microsoft.AspNetCore.RateLimiting`, cookie auth, authorization policies, antiforgery | 10.0.15 | all in-box besides JwtBearer |
| 12 Testing | xUnit 2.9.3, `Microsoft.AspNetCore.Mvc.Testing` (`WebApplicationFactory`), NSubstitute, `FakeTimeProvider`, BenchmarkDotNet, `coverlet` not required | xunit 2.9.3, Mvc.Testing 10.0.15, NSubstitute 6.2.0, TimeProvider.Testing 10.11.0, BenchmarkDotNet 0.15.8 | xUnit built-in `Assert` (FluentAssertions went commercial) |
| 13 Time and utilities | `DateTimeOffset`, `TimeZoneInfo`, `TimeProvider`, `PeriodicTimer`, `Guid.CreateVersion7`, `Random`/`RandomNumberGenerator`, `[GeneratedRegex]`, raw string literals/`string.Format` for templating, `BackgroundService` | BCL | Razor and templating engines are out of scope |
| 14 Packaging | `dotnet publish` (self-contained, single-file, trimmed, `-r linux-x64`), `Dockerfile` with the official SDK/ASP.NET images, GitHub Actions | SDK 10 | |

**Database for examples:** SQLite via `Microsoft.Data.Sqlite` (`Data Source=:memory:` kept open by one connection, or a temp file). PostgreSQL is a documented swap: connection string and `NpgsqlConnection`; the section compiles and only connects when `DATABASE_URL` is set.

## 5. Language-specific Teaching Notes

| Module | Emphasis | Typical pitfall |
|--------|----------|-----------------|
| 01 | classes vs structs vs records, nullable reference types, pattern matching/switch expressions, LINQ, generics with constraints, `IDisposable`/`using`, exceptions | `==` on reference types, mutating a `struct` copy, `null!` suppression, deferred LINQ execution (multiple enumeration) |
| 02 | the middleware pipeline order, `WebApplication` builder, DI lifetimes, route groups/endpoint filters, `Results.*`, binding sources, graceful shutdown/`IHostApplicationLifetime` | middleware order bugs, capturing scoped services in singletons, forgetting `await` |
| 03 | `TypedResults`, `ProblemDetails`, validation with `AddValidation`, pagination, versioning by route group, idempotency keys, OpenAPI | returning entities directly (over-posting), unbounded `pageSize` |
| 04 | STJ naming policy, `JsonSerializerContext`, `[JsonPropertyName]`, `JsonConverter<T>`, `required`, nullability and `JsonIgnoreCondition`, `DateTimeOffset` ISO 8601, `JsonDocument` | STJ is case-sensitive by default in raw usage (web defaults differ), missing members become defaults silently, polymorphism needs `[JsonDerivedType]` |
| 05 | `using` for connection/command/reader, parameters (`@name`), `DbTransaction`, `DBNull`, EF Core `DbContext` lifetime and change tracking, `AsNoTracking`, migrations concept | string-concatenated SQL, not disposing readers, `DbContext` registered as singleton, N+1 with lazy loading |
| 06 | `Task`/`ValueTask`, `async` all the way, `ConfigureAwait` mostly not needed in ASP.NET Core, `Channel<T>`, cancellation tokens, `lock` vs `SemaphoreSlim` | `.Result`/`.Wait()` deadlocks, `async void`, fire-and-forget exceptions, `lock` around an `await` (compile error) |
| 07 | `IHttpClientFactory`/typed clients, `HttpResponseMessage.EnsureSuccessStatusCode`, `HttpClient.Timeout` vs resilience timeouts, streaming with `ResponseHeadersRead` | `new HttpClient()` per request, `using` around a shared client, buffering huge bodies |
| 08 | `using`/`await using`, `FileStream` options, `Stream.CopyToAsync`, `Path.Combine` pitfalls, `Path.GetFullPath` traversal check, hashing a stream | `Path.Combine` with a rooted second argument discards the first, not flushing, sync I/O in async code |
| 09 | configuration provider order, `IOptions` vs `IOptionsSnapshot` vs `IOptionsMonitor`, `ValidateOnStart`, secrets never in appsettings | environment variable naming (`Section__Key`), binding silently leaving defaults |
| 10 | structured message templates (not interpolation), scopes, log levels per category, exception middleware + ProblemDetails, health checks | `logger.LogInformation($"...")` loses structure, logging PII/tokens |
| 11 | PBKDF2 parameters, `FixedTimeEquals`, JWT validation parameters, `[Authorize(Policy)]`/`RequireAuthorization`, rate limiter partitioning, antiforgery for cookie auth | disabling JWT validation to "make it work", `==` for secret comparison, fixed-window burst at boundary |
| 12 | `WebApplicationFactory<Program>` + `partial class Program`, replacing services, NSubstitute, `FakeTimeProvider`, `[Theory]` + `[InlineData]`, benchmarks in Release | shared state between tests via static, in-memory SQLite connection closed too early |
| 13 | `DateTimeOffset` over `DateTime`, `TimeProvider` for testability, `PeriodicTimer`, `BackgroundService` shutdown | `DateTime.Now` in servers, `Task.Delay` loops that drift |
| 14 | `dotnet publish -c Release -r linux-x64 --self-contained -p:PublishSingleFile=true`, multi-stage Dockerfile, `ASPNETCORE_URLS`, non-root `USER app` | trimming + reflection-based serialization (use source generators), copying `bin/` into the image |

## 6. Comparison Anchors
Java (classes/interfaces/checked vs unchecked exceptions, Spring DI), TypeScript/JS (`async`/`await`, LINQ vs array methods), Go (goroutines vs tasks), Python (decorators vs attributes).

## 7. Module Ordering Adjustments
Default order 01-14. Module 01 covers exceptions and `IDisposable` before HTTP. DI is introduced in module 02 and reused in 05, 07, 09 and 10. Module 06 (concurrency) is a prerequisite for the project P4 and P6.

## 8. Out of Scope for This Language
MVC controllers/Razor/Blazor, SignalR, gRPC, Entity Framework migrations tooling beyond a mention, `unsafe` code and pointers, source-generator authoring, reflection internals, Native AOT (mentioned in 14 only), Windows-specific APIs.
