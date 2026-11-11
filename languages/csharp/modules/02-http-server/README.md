# 02. HTTP Server in C#

> Estimated time: 2h | Difficulty: Intermediate | Prerequisites: [01 Language Fundamentals](../01-language-fundamentals/README.md)

## Learning Outcomes
After this module you can:
- [ ] Build a server with routes, path/query/header/body/form/file binding using ASP.NET Core Minimal APIs
- [ ] Write middleware, reason about pipeline order and short-circuiting
- [ ] Write responses: status codes, headers, cookies, streaming, content negotiation
- [ ] Configure CORS, static files, Kestrel limits/timeouts and graceful shutdown
- [ ] Organise endpoints with route groups, endpoint filters and dependency injection (and its lifetimes)

## 1. Concept Overview
An HTTP server turns a stream of requests into responses. The pieces are always the same: a **router** picks a
handler from method + path, **middleware** wraps every request (logging, ids, auth, CORS, errors), a handler
reads the request (route/query/header/body) and writes a response, and the host starts/stops the listener.

```
request -> [middleware 1 -> middleware 2 -> ... -> routing -> endpoint (+filters)] -> response
                 ^ each may short-circuit or act again on the way back out
```

## 2. How C# Approaches It

| Concept | ASP.NET Core 10 (in the shared framework, no extra package) |
|---------|------------------------------|
| Host / lifecycle | `WebApplication.CreateBuilder(args)` -> `builder.Build()` -> `app.RunAsync()` (handles Ctrl-C/SIGTERM) |
| Listener | Kestrel; `UseUrls`, `ConfigureKestrel(k => k.Limits...)` |
| Routing | `app.MapGet/MapPost/...`, `{id:int}` constraints, `MapGroup("/tasks")` |
| Binding | by parameter type and name: route, query, `[FromHeader]`, JSON body (a complex type), `[FromForm]`, `IFormFile`, services |
| Responses | return a value (JSON/text) or `Results.*` / `TypedResults.*` (`Ok`, `Created`, `NotFound`, `Stream`...) |
| Middleware | `app.Use(...)` inline, convention class with `InvokeAsync`, `app.UseMiddleware<T>()`, `UseWhen` |
| Cross-cutting per endpoint | endpoint filters (`AddEndpointFilter`) on endpoints or groups |
| DI | `builder.Services.AddSingleton/AddScoped/AddTransient`, parameters resolved automatically |
| Errors | `AddProblemDetails()` + `UseExceptionHandler()` (details are not leaked) |

Idiomatic: Minimal APIs with route groups and `TypedResults`; DI instead of statics; `CancellationToken` parameters.
Controllers (`[ApiController]`) exist and are what older code bases use, but are not taught here.

## 3. Guided Examples
One web project, one example per name. Start each with `ADDR=127.0.0.1:8080` (default) and stop it with Ctrl-C.
Commands below use `ADDR=127.0.0.1:8080` implicitly. Responses were captured from real runs (the `Date`/`Server` headers are omitted).

### 3.1 Hello, routes and binding
- **Goal:** routes, route/query parameters, route constraints, 404/405
- **Run:** `dotnet run --project modules/02-http-server/examples -- 01-hello`
- **Code:** [01_Hello.cs](examples/01_Hello.cs)
  ```csharp
  app.MapGet("/hello/{name}", (string name) => $"Hello, {name}!");
  app.MapGet("/greet", (string? who, int times = 1) => ...);   // query string, optional/defaulted
  app.MapGet("/users/{id:int}", (int id) => Results.Ok(new { id }));
  ```
- **Walkthrough:** parameter names bind to route values first, then the query string. A non-matching constraint
  means "no route" -> 404; a known path with another verb -> 405 with `Allow`.
- **Expected output:**
  ```
  $ curl -s localhost:8080/hello/Ada            ->  Hello, Ada!
  $ curl -s 'localhost:8080/greet?who=Bob&times=2'  ->  Hi Bob! Hi Bob!
  $ curl -s localhost:8080/greet                ->  Hi you!
  $ curl -s localhost:8080/users/7              ->  {"id":7}
  $ curl -si localhost:8080/users/abc           ->  HTTP/1.1 404 Not Found
  $ curl -si -X POST localhost:8080/ping        ->  HTTP/1.1 204 No Content
  $ curl -si -X POST localhost:8080/hello/x     ->  HTTP/1.1 405 Method Not Allowed   (Allow: GET)
  ```

### 3.2 Middleware pipeline
- **Goal:** order, short-circuiting, convention-based middleware, branching with `UseWhen`
- **Run:** `dotnet run --project modules/02-http-server/examples -- 02-middleware`
- **Code:** [02_MiddlewarePipeline.cs](examples/02_MiddlewarePipeline.cs)
  ```csharp
  app.Use(async (context, next) => { ...; await next(context); ... });   // code before/after the rest
  app.UseMiddleware<RequestIdMiddleware>();                              // class with InvokeAsync
  if (blocked) { context.Response.StatusCode = 403; return; }            // not calling next = short-circuit
  ```
- **Walkthrough:** each middleware records its name in an `X-Pipeline` header, so you can see the order.
  Headers must be set in `Response.OnStarting` because they cannot change once the body starts.
- **Expected output:**
  ```
  $ curl -si -H 'X-Request-ID: abc-123' localhost:8080/
  HTTP/1.1 200 OK
  Content-Type: text/plain; charset=utf-8
  X-Request-ID: abc-123
  X-Pipeline: outer-in > inner-in > endpoint

  request id: abc-123

  $ curl -si localhost:8080/admin/stats | grep -E '^(HTTP|X-Pipeline)'
  HTTP/1.1 200 OK
  X-Pipeline: outer-in > inner-in > admin-only > endpoint

  $ curl -si localhost:8080/blocked          # X-Request-ID is a random 32-hex id here
  HTTP/1.1 403 Forbidden
  X-Pipeline: outer-in

  blocked by middleware
  ```

### 3.3 Requests and responses
- **Goal:** every binding source and the common response shapes
- **Run:** `dotnet run --project modules/02-http-server/examples -- 03-request-response`
- **Code:** [03_RequestResponse.cs](examples/03_RequestResponse.cs)
- **Walkthrough:** JSON body binds to a record (malformed JSON -> 400 before your code runs); `[FromForm]` and
  `IFormFile` need `.DisableAntiforgery()` unless you wire up antiforgery (module 11); `Results.Stream` flushes chunks
  as produced; cookies are read from `Request.Cookies` and set via `Response.Cookies`; `/negotiate` inspects `Accept`.
- **Expected output:**
  ```
  $ curl -s -H 'X-Client: cli' 'localhost:8080/notes/5?format=short'
  {"id":5,"format":"short","client":"cli"}
  $ curl -s localhost:8080/notes/five -o /dev/null -w '%{http_code}\n'
  404
  $ curl -si -X POST -H 'Content-Type: application/json' -d '{"title":"Write docs","tags":["docs"]}' localhost:8080/notes
  HTTP/1.1 201 Created
  Content-Type: application/json; charset=utf-8
  Location: /notes/1

  {"id":1,"title":"Write docs","tags":["docs"]}
  $ curl -s -X POST -H 'Content-Type: application/json' -d '{"title":' localhost:8080/notes -o /dev/null -w '%{http_code}\n'
  400
  $ curl -s -X POST -d 'name=Ada&age=36' localhost:8080/form
  Ada is 36
  $ printf 'one\ntwo\n' | curl -s -F 'file=@-;filename=notes.txt;type=text/plain' localhost:8080/upload
  {"fileName":"notes.txt","length":8,"contentType":"text/plain","lines":2}
  $ curl -sN localhost:8080/stream
  chunk 1
  chunk 2
  chunk 3
  $ curl -si -c jar.txt localhost:8080/visit | grep -E '^(Set-Cookie|Cache)'
  Cache-Control: no-store
  Set-Cookie: visited=1; max-age=3600; path=/; samesite=lax; httponly
  $ curl -s -b jar.txt localhost:8080/visit
  visit 2
  $ curl -s localhost:8080/negotiate;  curl -s -H 'Accept: text/plain' localhost:8080/negotiate
  {"kind":"json"}
  plain text
  $ curl -s -H 'Accept: application/xml' localhost:8080/negotiate -o /dev/null -w '%{http_code}\n'
  406
  ```

### 3.4 CORS, static files, timeouts and graceful shutdown
- **Goal:** the production-facing knobs of the host
- **Run:** `dotnet run --project modules/02-http-server/examples -- 04-cors-static-shutdown`
- **Code:** [04_CorsStaticShutdown.cs](examples/04_CorsStaticShutdown.cs)
  ```csharp
  kestrel.Limits.RequestHeadersTimeout = TimeSpan.FromSeconds(10);
  builder.Services.Configure<HostOptions>(o => o.ShutdownTimeout = TimeSpan.FromSeconds(10));
  app.MapPost("/shutdown", (IHostApplicationLifetime host) => { host.StopApplication(); return Results.Accepted(); });
  ```
- **Walkthrough:** CORS is an allow-list: a matching `Origin` gets `Access-Control-Allow-Origin`, others get nothing
  (the browser blocks them; curl does not care). On shutdown Kestrel stops accepting connections but lets
  in-flight requests finish, up to `ShutdownTimeout`. Ctrl-C/SIGTERM does the same as `/shutdown`; the process exits with code 0.
- **Expected output:**
  ```
  $ curl -si localhost:8080/static/hello.txt | head -4
  HTTP/1.1 200 OK
  Content-Length: 26
  Content-Type: text/plain
  Accept-Ranges: bytes
  $ curl -si -H 'Origin: https://app.example.com' localhost:8080/api/ping | grep -i access-control
  Access-Control-Allow-Origin: https://app.example.com
  $ curl -si -H 'Origin: https://evil.example.com' localhost:8080/api/ping | grep -ic access-control
  0
  $ curl -si -X OPTIONS -H 'Origin: https://app.example.com' -H 'Access-Control-Request-Method: POST' \
         -H 'Access-Control-Request-Headers: content-type' localhost:8080/api/ping | grep -iE '^(HTTP|access-control)'
  HTTP/1.1 204 No Content
  Access-Control-Allow-Headers: Content-Type
  Access-Control-Allow-Methods: GET,POST
  Access-Control-Allow-Origin: https://app.example.com

  $ curl -s 'localhost:8080/slow?ms=1500' &  curl -s -X POST localhost:8080/shutdown;  sleep 0.3;  curl -s -m 1 localhost:8080/api/ping
  curl: (7) Failed to connect ...          <- new connections are refused
  done                                     <- the in-flight request still completed (after ~1.5 s)
  ```
  The server console prints `stopping: no new connections, draining requests` then `stopped`.

### 3.5 Task Tracker server: groups, filters, DI, error handler
- **Goal:** structure a small service
- **Run:** `dotnet run --project modules/02-http-server/examples -- 05-task-tracker-server`
- **Code:** [05_TaskTrackerServer.cs](examples/05_TaskTrackerServer.cs)
  ```csharp
  var tasks = app.MapGroup("/tasks");
  tasks.AddEndpointFilter(...);                         // runs for every endpoint in the group
  tasks.MapGet("/{id:int}", ...).AddEndpointFilter(PositiveId);
  builder.Services.AddScoped<ScopedDep>();              // one per request
  ```
- **Walkthrough:** the store is a singleton registered against an interface; `/lifetimes` numbers instances to show
  singleton (always 1), scoped (new per request, shared inside it) and transient (new per resolution).
  `UseExceptionHandler()` + `AddProblemDetails()` turns exceptions into a generic 500 `application/problem+json`.
  JSON is `snake_case` as the Task Tracker wire format demands.
- **Expected output (fresh server):**
  ```
  $ curl -s localhost:8080/lifetimes; curl -s localhost:8080/lifetimes
  {"singleton":1,"scoped":1,"scoped_again":1,"transient":[1,2]}
  {"singleton":1,"scoped":2,"scoped_again":2,"transient":[3,4]}
  $ curl -si -X POST -H 'Content-Type: application/json' \
      -d '{"title":"  Write docs ","due_date":"2030-01-31","tags":["docs","p1"]}' localhost:8080/tasks
  HTTP/1.1 201 Created
  Content-Type: application/json; charset=utf-8
  Location: /tasks/1
  X-Group: tasks

  {"id":1,"title":"Write docs","status":"todo","due_date":"2030-01-31","tags":["docs","p1"],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}
  $ curl -s -X POST -H 'Content-Type: application/json' -d '{"title":" "}' localhost:8080/tasks
  "title is required"                                  (400)
  $ curl -s localhost:8080/tasks/0
  "id must be positive"                                (400)
  $ curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/tasks/99
  404
  $ curl -s -o /dev/null -w '%{http_code} %{content_type}\n' localhost:8080/boom
  500 application/problem+json
  ```
  (Module 03 replaces the ad-hoc 400 strings with proper `ProblemDetails`.)

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| Authentication/CORS/exception middleware has no effect | order is the order of `Use...` calls; each only sees what comes after it | exception handler first, then static files/CORS/auth, endpoints last |
| `InvalidOperationException: Scoped service ... from root provider` or stale data | a singleton captured a scoped service (captive dependency) | inject `IServiceScopeFactory` and create a scope, or change lifetimes; scope validation is on in Development |
| Forgetting `await` (or returning `async void`): response completes early, exceptions vanish | `Task` is just a value; the compiler only warns (CS4014) | `await` everything, return `Task`, keep warnings as errors |
| Setting a header or status after the body started | `Response.HasStarted` becomes true on first write | set headers first or use `Response.OnStarting` |
| `[FromForm]` / `IFormFile` endpoint returns 400/throws about antiforgery | form binding requires antiforgery metadata by default | `UseAntiforgery()` + token, or `.DisableAntiforgery()` for non-browser APIs |
| Handler uses `HttpContext` after returning, or in a fire-and-forget task | `HttpContext` is pooled and reused | copy the values you need first, never capture the context |
| Using `HttpContext.RequestAborted` as "server stopping" | it only signals client disconnect | `IHostApplicationLifetime.ApplicationStopping` for host shutdown |
| Middleware reads `Request.Body` and the endpoint then sees an empty body (or a 400) | the body is a forward-only stream | `Request.EnableBuffering()` and reset `Position = 0` (or avoid reading it) |
| Listening only on `localhost` inside a container | default binds loopback | `ASPNETCORE_URLS=http://0.0.0.0:8080` / `ADDR` (module 14) |

## 5. Coming From Other Languages
- **Go (`net/http`):** `app.Use` is the `func(http.Handler) http.Handler` wrapper; `HttpContext` replaces `(w, r)`; the request
  `CancellationToken` plays the role of `r.Context()`; `http.Server.Shutdown` is built into `RunAsync`.
- **Express/Node:** `app.Use((ctx, next) => ...)` is `app.use((req, res, next) => ...)`, but you `await next(ctx)`.
- **Spring:** Minimal API endpoints are `@GetMapping` methods without classes; endpoint filters are interceptors; constructor injection is the same idea.
- **Python (FastAPI):** parameter binding by name/type is the same style; filters = dependencies.

## 6. Exercises
Tests start an in-memory server (`TestServer`, no sockets) so they are fast and offline.

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | Greeting routes | Easy | routing, route/query/header binding, status codes | `dotnet test modules/02-http-server/exercises --filter "FullyQualifiedName~.Ex1_"` |
| 2 | Request-ID middleware | Medium | middleware, `OnStarting`, `HttpContext.Items` | `dotnet test modules/02-http-server/exercises --filter "FullyQualifiedName~.Ex2_"` |
| 3 | Content negotiation | Stretch | `Accept` parsing, q-values, 406, `Vary` | `dotnet test modules/02-http-server/exercises --filter "FullyQualifiedName~.Ex3_"` |

### Exercise 1: Greeting routes ([Ex1_GreetingRoutes.cs](exercises/Ex1_GreetingRoutes.cs))
- **Task:** map `/hello`, `/hello/{name}`, `/add?a=&b=` and `/headers/{name}` as documented in the file.
- **Acceptance:** `Ex1_GreetingRoutesTests` (including 400 for bad operands, 404 for a missing header, 405 for POST).
- **Hint:** <details><summary>show</summary>Bind `a`/`b` as `string?` and use `int.TryParse`; take `HttpRequest` as a parameter to read headers.</details>

### Exercise 2: Request-ID middleware ([Ex2_RequestIdMiddleware.cs](exercises/Ex2_RequestIdMiddleware.cs))
- **Task:** implement `InvokeAsync`: reuse a safe incoming `X-Request-ID`, else generate one; expose it to handlers; send it on every response.
- **Acceptance:** `Ex2_RequestIdMiddlewareTests` (unsafe/overlong ids are replaced; 404 and 418 responses also carry the header).
- **Hint:** <details><summary>show</summary>`context.Response.OnStarting(() => ...)` runs just before the first byte is sent, whatever the endpoint does.</details>

### Exercise 3: Content negotiation ([Ex3_ExportRoutes.cs](exercises/Ex3_ExportRoutes.cs))
- **Task:** implement `GET /tasks/export` in JSON/CSV/plain text, choosing by `Accept` with q-values and specificity.
- **Acceptance:** `Ex3_ExportRoutesTests` (quality ordering, `text/*`, `q=0`, 406, `Vary: Accept`).
- **Hint:** <details><summary>show</summary>`Request.GetTypedHeaders().Accept` gives parsed `MediaTypeHeaderValue`s with `Quality`; compute a quality per supported type from its most specific matching range.</details>

## 7. Recap
- A Minimal API is `WebApplication` + `Map*` + middleware; binding is by parameter type and name.
- Middleware order is behaviour; not calling `next` short-circuits; set headers before the body starts.
- Use DI with the right lifetime (singleton state, scoped per request, transient cheap) and never capture scoped in singleton.
- Graceful shutdown, timeouts, CORS and static files are host configuration, not handler code.
- Use route groups + filters for shared concerns; reach for controllers only when maintaining older code.

## 8. Self-check Questions
1. Why does the response header `X-Pipeline` not contain `outer-out` in example 3.2?
2. What status do you get for `GET /users/abc` with `{id:int}`, and for `POST /hello/x`? Why different?
3. A singleton service needs a per-request value. What are two correct ways to get it?
4. What happens to an in-flight request when you press Ctrl-C, and which setting bounds the wait?
5. Why can a browser fail a request that `curl` completes against the same CORS-configured server?

## 9. Further Reading
- [Minimal APIs overview](https://learn.microsoft.com/aspnet/core/fundamentals/minimal-apis/overview)
- [ASP.NET Core middleware](https://learn.microsoft.com/aspnet/core/fundamentals/middleware/)
- [Kestrel options and limits](https://learn.microsoft.com/aspnet/core/fundamentals/servers/kestrel/options)
- [Host shutdown](https://learn.microsoft.com/dotnet/core/extensions/generic-host#host-shutdown)

---
**Next:** [03 REST API Design](../03-rest-api-design/README.md)
