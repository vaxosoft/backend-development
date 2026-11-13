# 02. HTTP Server in TypeScript

> Estimated time: 2 h | Difficulty: Intermediate | Prerequisites: [01 Language Fundamentals](../01-language-fundamentals/README.md)

## Learning Outcomes

After this module you can:

- [ ] Route requests and read path parameters, query strings, headers and JSON/form/multipart bodies
- [ ] Write responses: status codes, headers, JSON, plain text and streams
- [ ] Structure a server with hooks, plugins and decorators, and explain Fastify's encapsulation
- [ ] Add CORS, cookies, content negotiation and static files
- [ ] Configure server timeouts and shut the server down gracefully on SIGINT/SIGTERM

## 1. Concept Overview

An HTTP server accepts TCP connections, parses requests (method, target, headers, body), picks a handler by method and path, and writes a response. Around the handler sits a _pipeline_: cross-cutting steps such as request ids, authentication, CORS or logging that run before and after every handler (middleware).

Production servers also need limits (header/body/request timeouts, maximum body size) and a _graceful shutdown_: stop accepting connections, let in-flight requests finish, then exit. Everything else in the backend path (REST design, auth, logging) builds on this lifecycle.

## 2. How TypeScript Approaches It

- **`node:http`** is the built-in server. It gives you raw requests and responses and no router, so you parse the URL and write headers yourself. Good for understanding, rarely used directly.
- **Fastify 5** (chosen in [PROFILE.md](../../PROFILE.md)) is a small, fast, plugin-based framework with routing, hooks, content-type parsers, a built-in `inject()` for tests and an encapsulation model. Official plugins used here: `@fastify/cors`, `@fastify/cookie`, `@fastify/multipart`, `@fastify/static`.
- **Handlers are `async` functions.** Return a value (sent as JSON) or `return reply.code(...).send(...)`. Never mix a returned value with `reply.send` in the same handler.
- **Lifecycle:** `onRequest` -> parsing -> `preValidation` -> `preHandler` -> handler -> `onSend` -> `onResponse`. Hooks run in registration order within a scope.
- **Encapsulation:** `app.register(plugin)` creates a child scope. Hooks and decorators added inside stay inside, unless the plugin is wrapped with `fastify-plugin` (`fp`).
- Express-style `(req, res, next)` middleware is out of scope: hooks replace it.

Examples read the listen address from `ADDR` (`host:port`, default `127.0.0.1:8080`) and stop cleanly on Ctrl-C/SIGTERM via [examples/_shared.ts](examples/_shared.ts).

## 3. Guided Examples

Start a server in one terminal, run the `curl` commands in another. The `Date`, `Connection` and `Keep-Alive` response headers are omitted below.

### 3.1 The raw `node:http` server

- **Goal:** see what a framework does for you: URL parsing, manual routing, body reading, timeouts.
- **Run:** `node modules/02-http-server/examples/01_node_http.ts`
- **Code:** [examples/01_node_http.ts](examples/01_node_http.ts)
- **Walkthrough:** the request listener parses `req.url` with `new URL`, matches paths with `if`/regex, reads the body by iterating the request stream (with a size limit), and answers 404/405 itself. Because the listener is `async`, it must `catch` its own errors: nobody else will. `requestTimeout`, `headersTimeout` and `keepAliveTimeout` protect against slow clients. Shutdown calls `server.close()` and `closeIdleConnections()`.

```ts
server.requestTimeout = 30_000;
server.headersTimeout = 10_000;
server.keepAliveTimeout = 5_000;
```

- **Expected output:**

  ```text
  $ curl -i http://127.0.0.1:8080/hello/Ada
  HTTP/1.1 200 OK
  content-type: text/plain; charset=utf-8
  content-length: 12

  Hello, Ada!
  $ curl -i "http://127.0.0.1:8080/hello/Ada%20L?shout=true"
  HTTP/1.1 200 OK
  content-type: text/plain; charset=utf-8
  content-length: 14

  HELLO, ADA L!
  $ curl -i -X POST -d ping http://127.0.0.1:8080/echo
  HTTP/1.1 200 OK
  content-type: text/plain; charset=utf-8
  content-length: 4

  ping
  $ curl -i -X DELETE http://127.0.0.1:8080/echo
  HTTP/1.1 405 Method Not Allowed
  allow: POST
  content-type: text/plain; charset=utf-8
  content-length: 19

  method not allowed
  ```

### 3.2 Fastify basics

- **Goal:** routes, params, query, headers, JSON bodies and status codes.
- **Run:** `node modules/02-http-server/examples/02_fastify_basics.ts`
- **Code:** [examples/02_fastify_basics.ts](examples/02_fastify_basics.ts)
- **Walkthrough:** returning an object sends JSON with the right `content-type`. `app.get<{ Params: { name: string } }>` types the parameters, but the types are only _claims_: query values are always strings, and `/page?page=abc` shows `Number('abc')` is `NaN`, which JSON turns into `null`. The default 404 and the invalid-JSON 400 come from Fastify.

```ts
app.post<{ Body: { message?: unknown } }>('/echo', async (req, reply) => {
  return reply.code(201).header('x-echo', 'true').send({ received: req.body });
});
```

- **Expected output:**

  ```text
  $ curl -i http://127.0.0.1:8080/healthz
  HTTP/1.1 200 OK
  content-type: application/json; charset=utf-8
  content-length: 15

  {"status":"ok"}
  $ curl -i http://127.0.0.1:8080/hello
  HTTP/1.1 200 OK
  content-type: text/plain
  content-length: 13

  Hello, world!
  $ curl http://127.0.0.1:8080/page
  {"page":1,"valid":true}
  $ curl http://127.0.0.1:8080/page?page=abc
  {"page":null,"valid":false}
  $ curl -H 'user-agent: lesson' -H 'accept: application/json' http://127.0.0.1:8080/whoami
  {"userAgent":"lesson","accept":"application/json"}
  $ curl -i -X POST -H 'content-type: application/json' -d '{"message":"hi"}' http://127.0.0.1:8080/echo
  HTTP/1.1 201 Created
  x-echo: true
  content-type: application/json; charset=utf-8
  content-length: 29

  {"received":{"message":"hi"}}
  $ curl -i -X POST -H 'content-type: application/json' -d '{oops' http://127.0.0.1:8080/echo
  HTTP/1.1 400 Bad Request
  content-type: application/json; charset=utf-8
  content-length: 160

  {"statusCode":400,"code":"FST_ERR_CTP_INVALID_JSON_BODY","error":"Bad Request","message":"Body is not valid JSON but content-type is set to 'application/json'"}
  ```

### 3.3 Plugins, hooks, decorators, CORS, cookies, negotiation

- **Goal:** structure cross-cutting behaviour and understand encapsulation.
- **Run:** `node modules/02-http-server/examples/03_plugins_hooks_cors_cookies.ts`
- **Code:** [examples/03_plugins_hooks_cors_cookies.ts](examples/03_plugins_hooks_cors_cookies.ts)
- **Walkthrough:** `requestId` is wrapped in `fp`, so its decorator and hooks apply to the whole app; it is registered first because CORS answers preflight requests in its own `onRequest` hook and hooks run in registration order. `adminRoutes` is _not_ wrapped, so its `onRequest` hook guards only `/admin/*`; returning the reply from a hook ends the request. Cookies are set with `HttpOnly` and `SameSite`. `/tasks` picks JSON or CSV from `Accept` and answers `406` otherwise.

```ts
const requestId = fp(async (app) => {
  app.decorateRequest('requestId', '');
  app.addHook('onRequest', async (req) => {
    /* echo or generate */
  });
  app.addHook('onSend', async (req, reply) => {
    reply.header('x-request-id', req.requestId);
  });
});
```

- **Expected output:**

  ```text
  $ curl -i -H 'x-request-id: demo-1' -H 'origin: https://app.example.com' http://127.0.0.1:8080/ping
  HTTP/1.1 200 OK
  vary: Origin
  access-control-allow-origin: https://app.example.com
  content-type: application/json; charset=utf-8
  x-request-id: demo-1

  {"pong":true,"requestId":"demo-1"}
  $ curl -i -H 'x-request-id: demo-4' http://127.0.0.1:8080/admin/stats
  HTTP/1.1 403 Forbidden
  vary: Origin
  content-type: application/json; charset=utf-8
  x-request-id: demo-4

  {"error":"admins only"}
  $ curl -i -X OPTIONS -H 'x-request-id: demo-3' -H 'origin: https://app.example.com' -H 'access-control-request-method: GET' http://127.0.0.1:8080/ping
  HTTP/1.1 204 No Content
  vary: Origin, Access-Control-Request-Headers
  access-control-allow-origin: https://app.example.com
  access-control-allow-methods: GET,HEAD,POST
  x-request-id: demo-3

  $ curl -i -H 'x-request-id: demo-5' http://127.0.0.1:8080/visit
  HTTP/1.1 200 OK
  vary: Origin
  content-type: application/json; charset=utf-8
  x-request-id: demo-5
  set-cookie: visits=1; Path=/; HttpOnly; SameSite=Lax

  {"visits":1}
  $ curl -i -H 'accept: text/csv' -H 'x-request-id: demo-6' http://127.0.0.1:8080/tasks
  HTTP/1.1 200 OK
  vary: accept
  content-type: text/csv; charset=utf-8
  x-request-id: demo-6

  id,title,status
  1,Write docs,todo
  2,Ship it,done
  $ curl -i -H 'accept: text/xml' -H 'x-request-id: demo-7' http://127.0.0.1:8080/tasks
  HTTP/1.1 406 Not Acceptable
  vary: accept
  content-type: application/json; charset=utf-8
  x-request-id: demo-7

  {"error":"supported: application/json, text/csv"}
  ```

  A request from another origin (`origin: https://evil.example`) receives no `access-control-*` header at all.

### 3.4 Forms, multipart uploads, static files, streaming

- **Goal:** the other request/response body kinds.
- **Run:** `node modules/02-http-server/examples/04_forms_uploads_static.ts`
- **Code:** [examples/04_forms_uploads_static.ts](examples/04_forms_uploads_static.ts) and the [public/](examples/public/hello.txt) directory
- **Walkthrough:** Fastify parses only JSON and text by default, so a small `addContentTypeParser` handles `application/x-www-form-urlencoded` with `URLSearchParams`. `req.file()` yields a _stream_: the handler hashes it chunk by chunk, so memory stays flat. When the `fileSize` limit is hit the stream is silently truncated; you must check `upload.file.truncated` and answer 413. `@fastify/static` maps `/static/` to a directory and refuses `..` traversal. A `Readable` passed to `reply.send` is streamed with chunked encoding.

```ts
if (upload.file.truncated) return reply.code(413).send({ error: 'file exceeds 1 MiB' });
```

- **Expected output:**

  ```text
  $ curl -d 'user=ada&pw=secret' http://127.0.0.1:8080/login
  {"form":{"user":"ada","pw":"secret"}}
  $ curl -i -F 'file=@modules/02-http-server/examples/public/hello.txt' http://127.0.0.1:8080/upload
  HTTP/1.1 201 Created
  content-type: application/json; charset=utf-8
  content-length: 126

  {"name":"hello.txt","type":"text/plain","size":26,"sha256":"51d84bc6f076d4419ab0d83ee1d5667a96e8df4fd5dade0fdcaad0e85ade2949"}
  $ curl -F 'other=x' http://127.0.0.1:8080/upload
  {"error":"multipart field \"file\" is required"}
  $ head -c 2000000 /dev/zero | curl -i -F 'file=@-;filename=big.bin' http://127.0.0.1:8080/upload
  HTTP/1.1 413 Payload Too Large
  content-type: application/json; charset=utf-8
  content-length: 30

  {"error":"file exceeds 1 MiB"}
  $ curl http://127.0.0.1:8080/static/hello.txt
  Hello from a static file.
  $ curl --path-as-is -i http://127.0.0.1:8080/static/../04_forms_uploads_static.ts
  HTTP/1.1 403 Forbidden
  $ curl "http://127.0.0.1:8080/numbers?count=3"
  1
  2
  3
  ```

  (`--path-as-is` stops curl from normalising the `..` away, so the server sees it.) A `100 Continue` line may precede the 413 response.

### 3.5 Task Tracker server with timeouts and graceful shutdown

- **Goal:** a realistic server: CRUD-ish routes, server timeouts, `close()` that waits for in-flight work.
- **Run:** `node modules/02-http-server/examples/05_task_server.ts`
- **Code:** [examples/05_task_server.ts](examples/05_task_server.ts)
- **Walkthrough:** `Fastify({ connectionTimeout, keepAliveTimeout, requestTimeout, forceCloseConnections: 'idle' })` configures the limits. `POST /tasks` answers `201` with a `Location` header, `DELETE` answers `204`. On SIGINT/SIGTERM `app.close()` stops the listener, closes idle keep-alive sockets and waits for busy requests. Validation here is deliberately minimal; [module 03](../03-rest-api-design/README.md) does it properly.
- **Expected output** (start `GET /slow?ms=2000` in one terminal, send Ctrl-C to the server in another; the slow request still completes and new connections are refused):

  ```text
  $ curl -i -X POST -H 'content-type: application/json' -d '{"title":"Ship it"}' http://127.0.0.1:8080/tasks
  HTTP/1.1 201 Created
  location: /tasks/2
  content-type: application/json; charset=utf-8
  content-length: 120

  {"id":2,"title":"Ship it","status":"todo","due_date":null,"tags":[],"owner_id":null,"created_at":"2030-01-01T09:00:00Z"}
  $ curl -i http://127.0.0.1:8080/tasks/99
  HTTP/1.1 404 Not Found
  content-type: application/json; charset=utf-8
  content-length: 26

  {"error":"task not found"}
  $ curl -i -X DELETE http://127.0.0.1:8080/tasks/2
  HTTP/1.1 204 No Content

  $ curl http://127.0.0.1:8080/slow?ms=2000          # terminal A
  {"done":true}
  ```

  Server terminal:

  ```text
  listening
  SIGTERM received, shutting down
  closed: no more connections
  bye
  ```

## 4. Common Pitfalls

| Mistake                                                                            | Why it happens                                                                          | Fix                                                                                                        |
| ---------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| Handler never answers, or answers twice (`Reply was already sent`)                 | mixing `return value` with `reply.send`, or sending inside a callback and returning too | in `async` handlers `return reply.send(...)` or return the value, never both                               |
| Unhandled rejection kills the process in a raw `node:http` listener                | an `async` listener's rejection is not caught by anyone                                 | wrap the body in `try`/`catch` (Fastify does it for you)                                                   |
| Hook/decorator is "missing" outside its plugin, or applies everywhere unexpectedly | plugins are encapsulated unless wrapped with `fastify-plugin`                           | use `fp` for shared behaviour, plain plugins for scoped behaviour                                          |
| `listen({ host: 'localhost' })` unreachable from `127.0.0.1` or IPv6 clients       | `localhost` may resolve to `::1` only                                                   | listen on an explicit `127.0.0.1` (or `0.0.0.0` in containers)                                             |
| Typed query/params trusted as numbers                                              | `Querystring: { page: number }` is only a claim; values are strings                     | convert and validate (module 03)                                                                           |
| Upload "succeeds" with a cut-off file                                              | `@fastify/multipart` truncates at `limits.fileSize` when you stream                     | check `file.truncated`, answer 413                                                                         |
| Shutdown hangs forever                                                             | idle keep-alive sockets keep `server.close()` pending                                   | `forceCloseConnections: 'idle'` or `server.closeIdleConnections()`, plus a deadline that destroys the rest |
| Reading the whole body into memory                                                 | `await req.file()` + `toBuffer()` or collecting chunks without a limit                  | stream chunk by chunk, set `bodyLimit`/`fileSize`                                                          |
| CORS "does not work" for preflights                                                | the `OPTIONS` request is answered before your hooks ran                                 | register CORS/request-id plugins in the right order, test with `OPTIONS`                                   |

## 5. Coming From Other Languages

- **Go `net/http`:** `http.Handler` ~ the `node:http` request listener; `ServeMux` has no built-in equivalent in Node, which is why Fastify's router exists. `Server.Shutdown(ctx)` ~ `server.close()` plus your own deadline.
- **Express (JavaScript):** `app.use` middleware ~ hooks; `next()` does not exist, a hook just returns (or sends a reply to stop).
- **Java/Spring, C#/ASP.NET:** filters and interceptors ~ hooks; DI containers ~ `decorate`; controllers are plain route functions in plugins.
- **Python FastAPI:** `Depends` ~ hooks/decorators; request validation is opt-in and done with zod here (module 03).

## 6. Exercises

Tests use `app.inject()` or a loopback listener on a free port; no external network.

| #   | Title                | Difficulty | Skill practiced                                          | Run tests                                                                    |
| --- | -------------------- | ---------- | -------------------------------------------------------- | ---------------------------------------------------------------------------- |
| 1   | Routes and responses | Easy       | routing, params, query, status codes, 404 handler        | `node --test "modules/02-http-server/exercises/ex1_routes/**/*.test.ts"`     |
| 2   | Request-id plugin    | Medium     | hooks, decorators, `fp` vs encapsulation                 | `node --test "modules/02-http-server/exercises/ex2_request_id/**/*.test.ts"` |
| 3   | Server lifecycle     | Stretch    | `node:http`, timeouts, graceful shutdown with a deadline | `node --test "modules/02-http-server/exercises/ex3_lifecycle/**/*.test.ts"`  |

### Exercise 1: Routes and responses

- **Task:** implement `buildApp()` in [routes.ts](exercises/ex1_routes/routes.ts) (`/healthz`, `/hello`, `/hello/:name`, `/items/:id`, `POST /echo`, JSON 404).
- **Acceptance:** [routes.test.ts](exercises/ex1_routes/routes.test.ts) passes. Note the tests for ids such as `1.5` and the empty id.
- **Hint:** <details><summary>Show</summary>`Number('')` is `0`; check the text with a regex before converting.</details>

### Exercise 2: Request-id plugin

- **Task:** implement `requestIdPlugin` (shared, via `fp`) and `adminPlugin` (encapsulated) in [request-id.ts](exercises/ex2_request_id/request-id.ts).
- **Acceptance:** [request-id.test.ts](exercises/ex2_request_id/request-id.test.ts) passes: echo valid ids, replace unsafe ones, set the header on 404s, protect `/admin` without affecting `/public`.
- **Hint:** <details><summary>Show</summary>`decorateRequest('requestId', '')` first; a hook that sends a reply must `return` it.</details>

### Exercise 3: Server lifecycle

- **Task:** implement `startServer` in [server.ts](exercises/ex3_lifecycle/server.ts): apply timeouts, listen, and provide an idempotent `shutdown(graceMs)`.
- **Acceptance:** [server.test.ts](exercises/ex3_lifecycle/server.test.ts) passes: in-flight requests finish, new connections are refused (`ECONNREFUSED`), idle and just-finished keep-alive sockets do not block shutdown, and stuck connections are destroyed after the grace period.
- **Hint:** <details><summary>Show</summary>`server.close(cb)`, `closeIdleConnections()`, `closeAllConnections()`; a connection that is busy when you close becomes idle after its response.</details>

## 7. Recap

- A server is a pipeline: parse -> hooks -> handler -> hooks -> write; Fastify hooks and plugins replace middleware.
- Types do not validate: path, query and body values need runtime checks.
- Stream large bodies, limit sizes, set timeouts.
- Graceful shutdown = stop accepting, finish in-flight, close idle sockets, enforce a deadline.
- Use `node:http` to learn, Fastify to build; use `inject()` to test without sockets.

## 8. Self-check Questions

1. What is the difference between registering a plugin with and without `fastify-plugin`?
2. Why must the request-id plugin be registered before `@fastify/cors`?
3. Why can `server.close()` hang even when no request is running?
4. What happens to an upload that exceeds `limits.fileSize`, and what must your handler do?
5. Why is `Querystring: { page: number }` misleading?

## 9. Further Reading

- [Fastify documentation](https://fastify.dev/docs/latest/): Lifecycle, Hooks, Plugins, Encapsulation
- [Node.js `http` API](https://nodejs.org/api/http.html): `Server`, timeouts, `closeIdleConnections`
- [MDN: HTTP](https://developer.mozilla.org/en-US/docs/Web/HTTP): methods, status codes, CORS, caching

---

**Next:** [03 REST API Design](../03-rest-api-design/README.md)
