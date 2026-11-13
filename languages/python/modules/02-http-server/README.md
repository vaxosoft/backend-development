# 02. HTTP Server in Python

> Estimated time: 2h | Difficulty: Beginner-Intermediate | Prerequisites: [01 Language Fundamentals](../01-language-fundamentals/README.md)

## Learning Outcomes
After this module you can:
- [ ] Explain the request/response cycle and what a server framework adds on top of sockets
- [ ] Route requests and bind path, query, header, form, cookie and JSON body parameters from type hints
- [ ] Write responses: status codes, headers, cookies, streaming bodies, simple content negotiation
- [ ] Add middleware (request ids, error recovery, CORS), dependencies, lifespan hooks and static files
- [ ] Shut a server down gracefully and configure timeouts

## 1. Concept Overview
An HTTP server accepts TCP connections, parses requests (method, path, headers, body), calls a *handler* chosen by a router, and writes a response (status, headers, body). *Middleware* wraps every handler (logging, request ids, CORS, error recovery). A production server also needs timeouts (so slow clients cannot hold connections forever) and *graceful shutdown* (stop accepting, finish in-flight requests, then exit).

```
client -> [ server (uvicorn) -> middleware stack -> router -> dependencies -> handler ] -> response
```

## 2. How Python Approaches It
- **ASGI** is the standard interface between servers and async Python apps. **uvicorn** is the server; **FastAPI** (built on **Starlette**) is the framework. You rarely touch raw ASGI.
- **Type hints drive binding:** a parameter named in the path is a path parameter, a scalar elsewhere is a query parameter, a Pydantic model is the JSON body, `Header()`, `Cookie()`, `Form()`, `UploadFile` pick the other sources. Conversion and validation failures become `422` automatically.
- **`def` vs `async def`:** a plain `def` handler runs in a worker thread (blocking calls are fine); an `async def` handler runs on the event loop and must never block.
- **`Depends`** injects shared objects (stores, settings, auth) and makes them replaceable in tests.
- **`lifespan`** is the startup/shutdown hook; **`app.mount`** serves static files.
- **Libraries (PROFILE.md):** `fastapi`, `uvicorn`, `python-multipart` (forms and uploads). The stdlib `http.server` is shown only to demonstrate what the framework hides; do not use it in production.

| Concern | stdlib `http.server` | FastAPI + uvicorn |
|---------|----------------------|-------------------|
| Routing | `if path == ...` chains | decorators, path templates |
| Parsing | manual (`Content-Length`, `json.loads`) | declared by type hints |
| Concurrency | one thread per connection | event loop + threadpool |
| Shutdown | wire up signals yourself | built in (SIGINT/SIGTERM) |

## 3. Guided Examples
Every server example reads `ADDR` (`host:port`, default `127.0.0.1:8080`) and exits cleanly on Ctrl-C or SIGTERM. Start one in a terminal and use `curl` from another. All commands run from `languages/python/`.

### 3.1 A server with only the standard library
- **Goal:** see sockets, parsing and routing without a framework.
- **Run:** `python modules/02-http-server/examples/01_stdlib_server.py`
- **Code:** [examples/01_stdlib_server.py](examples/01_stdlib_server.py)
- **Walkthrough:** `ThreadingHTTPServer` handles each connection in a thread. `do_GET`/`do_POST` dispatch by hand; every reply sets `Content-Length`, which keep-alive needs. For shutdown, signal handlers set an `Event`; the main thread then calls `server.shutdown()` (it must not run on the serving thread).

```python
stop = threading.Event()
signal.signal(signal.SIGTERM, lambda *_: stop.set())
```

- **Expected output:**

  ```
  $ curl -s localhost:8080/healthz
  {"status": "ok"}
  $ curl -s "localhost:8080/hello?name=Ada"
  Hello, Ada!
  $ curl -s localhost:8080/hello/Bob
  Hello, Bob!
  $ curl -s -X POST localhost:8080/echo -d '{"a":1}'
  {"you_sent": {"a": 1}}
  $ curl -s -o /dev/null -w '%{http_code}\n' -X POST localhost:8080/echo -d 'not json'
  400
  ```

### 3.2 FastAPI routing and parameter binding
- **Goal:** path, query and header parameters, validation, and `def` vs `async def`.
- **Run:** `python modules/02-http-server/examples/02_fastapi_routing.py`
- **Code:** [examples/02_fastapi_routing.py](examples/02_fastapi_routing.py)
- **Walkthrough:** `user_id: int` converts the path segment; `Path(ge=1)` and `Query(ge=1, le=50)` add constraints; `list[str] | None` with `Query()` accepts a repeated parameter; `Header()` maps `user_agent` to `User-Agent`. A body model (`Echo`) is parsed from JSON. The two `/blocking-*` routes show the safe ways to do blocking work.

```python
@app.get("/search")
def search(
    q: str,
    tag: Annotated[list[str] | None, Query()] = None,
    limit: Annotated[int, Query(ge=1, le=50)] = 10,
) -> dict[str, object]: ...
```

- **Expected output:**

  ```
  $ curl -s localhost:8080/hello/Ada
  Hello, Ada!
  $ curl -s localhost:8080/users/3/tasks/7
  {"user_id":3,"task_id":7}
  $ curl -s localhost:8080/users/x/tasks/7
  {"detail":[{"type":"int_parsing","loc":["path","user_id"],"msg":"Input should be a valid integer, unable to parse string as an integer","input":"x"}]}
  $ curl -s -H 'User-Agent: demo/1.0' "localhost:8080/search?q=docs&tag=a&tag=b&limit=5"
  {"q":"docs","tags":["a","b"],"limit":5,"user_agent":"demo/1.0"}
  $ curl -s "localhost:8080/search?limit=500"
  {"detail":[{"type":"missing","loc":["query","q"],"msg":"Field required","input":null},{"type":"less_than_equal","loc":["query","limit"],"msg":"Input should be less than or equal to 50","input":"500","ctx":{"le":50}}]}
  $ curl -s -X POST localhost:8080/echo -H 'Content-Type: application/json' -d '{"message":"hi","times":3}'
  {"echo":"hi hi hi"}
  ```

  The default 422 body (`detail`) is replaced by the shared error envelope in [module 03](../03-rest-api-design/README.md).

### 3.3 Requests and responses: forms, uploads, cookies, headers, streaming
- **Goal:** read every kind of input and write every kind of output.
- **Run:** `python modules/02-http-server/examples/03_requests_and_responses.py`
- **Code:** [examples/03_requests_and_responses.py](examples/03_requests_and_responses.py)
- **Walkthrough:** `Form()` needs `python-multipart`. `Response` as a parameter lets you set cookies and headers while still returning a normal value. `UploadFile` is spooled to disk when large; read it in chunks. `/report` inspects `Accept` and returns CSV, JSON or `406`. `StreamingResponse` takes any iterator, so a generator streams without building the body in memory.

```python
while chunk := await file.read(CHUNK):
    digest.update(chunk)
```

- **Expected output:**

  ```
  $ curl -si -d 'username=ada&password=pw' localhost:8080/login | grep -iE '^(set-cookie|\{)'
  set-cookie: session=s-ada; HttpOnly; Max-Age=3600; Path=/; SameSite=lax
  {"user":"ada"}
  $ curl -s -H 'Cookie: session=s-ada' localhost:8080/me
  {"session":"s-ada"}
  $ printf 'hello upload' | curl -s -F 'file=@-;filename=notes.txt;type=text/plain' localhost:8080/upload
  {"filename":"notes.txt","content_type":"text/plain","size":12,"sha256":"2d119f1cd272958a492a144af600b9dc36531f73027b34073967345b027021b1"}
  $ curl -si -X POST localhost:8080/tasks | grep -iE '^(HTTP|location|x-demo)'
  HTTP/1.1 201 Created
  location: /tasks/42
  x-demo: created
  $ curl -s -H 'Accept: text/csv' localhost:8080/report
  id,title
  1,Write docs
  2,Ship
  $ curl -s -H 'Accept: application/xml' localhost:8080/report
  {"error":{"code":"not_acceptable","message":"use application/json or text/csv"}}
  $ curl -sN "localhost:8080/stream?count=3"
  line 0
  line 1
  line 2
  ```

### 3.4 Middleware, CORS, dependencies, lifespan, static files, shutdown
- **Goal:** cross-cutting concerns and the server lifecycle.
- **Run:** `python modules/02-http-server/examples/04_middleware_and_lifespan.py`
- **Code:** [examples/04_middleware_and_lifespan.py](examples/04_middleware_and_lifespan.py)
- **Walkthrough:** the *last* middleware added is the *outermost*: requests enter `outer` then `inner`, responses leave `inner` then `outer` (CORS was added last, so it wraps everything). `require_api_key` is a dependency that raises `HTTPException`. `lifespan` prints around the server's life. `uvicorn.run(..., timeout_keep_alive=5, timeout_graceful_shutdown=10)` (in [m02_helpers.py](examples/m02_helpers.py)) sets idle and drain limits.

```python
app.middleware("http")(tag_middleware("inner"))
app.middleware("http")(tag_middleware("outer"))  # added last = outermost
```

- **Expected output:**

  ```
  $ curl -si localhost:8080/trail | grep -iE '^(x-order|\{)'
  x-order: inner,outer
  {"request_order":["outer","inner"]}
  $ curl -s -w ' [%{http_code}]\n' localhost:8080/private
  {"detail":"missing or invalid API key"} [401]
  $ curl -s -H 'X-API-Key: secret' localhost:8080/private
  {"key_length":"6"}
  $ curl -s localhost:8080/static/hello.txt
  Static files are served as-is.
  $ curl -si -X OPTIONS localhost:8080/tasks -H 'Origin: https://app.example.com' \
      -H 'Access-Control-Request-Method: POST' | grep -i allow-origin
  access-control-allow-origin: https://app.example.com
  ```

  Graceful shutdown: start `curl -s -w ' [%{http_code}]\n' "localhost:8080/slow?ms=1500"`, then send `kill -TERM <pid>` while it runs. The in-flight request completes and new connections are refused:

  ```
  new connection: curl exit=7
  {"status":"done"} [200]
  ```

  (The first line comes from `curl -m 2 -w 'new connection: curl exit=%{exitcode}\n' localhost:8080/readyz`, run just after the signal.)

### 3.5 Task Tracker over HTTP
- **Goal:** a small realistic server with dependency injection and a request-id middleware.
- **Run:** `python modules/02-http-server/examples/05_task_server.py`
- **Code:** [examples/05_task_server.py](examples/05_task_server.py)
- **Walkthrough:** `create_app(store)` builds the app (a factory, so tests can create isolated instances). `get_store` is injected with `Depends`. `POST /tasks` returns `201` and `Location`. Unknown ids raise `HTTPException(404)`. Task fields here are a subset (no `created_at`) to keep output deterministic; module 03 adds the full model and the shared error envelope.

- **Expected output:**

  ```
  $ curl -si -X POST localhost:8080/tasks -H 'Content-Type: application/json' \
      -H 'X-Request-ID: demo-1' -d '{"title":"Write docs","tags":["docs"]}' | grep -iE '^(HTTP|location|x-request-id|\{)'
  HTTP/1.1 201 Created
  location: /tasks/1
  x-request-id: demo-1
  {"id":1,"title":"Write docs","status":"todo","tags":["docs"]}
  $ curl -s localhost:8080/tasks/1
  {"id":1,"title":"Write docs","status":"todo","tags":["docs"]}
  $ curl -s localhost:8080/tasks/9
  {"detail":"task 9 not found"}
  $ curl -s -X POST localhost:8080/tasks -H 'Content-Type: application/json' -d '{"title":""}'
  {"detail":[{"type":"string_too_short","loc":["body","title"],"msg":"String should have at least 1 character","input":"","ctx":{"min_length":1}}]}
  ```

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `async def` handler calls `time.sleep`, `requests`, `sqlite3` or heavy CPU code and every request stalls | it runs on the single event-loop thread | use a plain `def` handler (threadpool) or `await run_in_threadpool(...)` / an async library |
| Forgetting `await` on a coroutine (e.g. `file.read()`) | the call returns a coroutine object that never runs | `await` it; `mypy --strict` and Python's "never awaited" warning catch this |
| Returning an ORM/internal object or a dict with secrets | without a return type or `response_model`, everything is serialised | declare the return type or `response_model` so only those fields are exposed |
| Middleware order surprises (CORS headers missing on errors, request id missing on 500) | the last `add_middleware` is outermost; `ServerErrorMiddleware` sits outside all of them | add CORS last; catch exceptions inside your own middleware if the response must carry its headers |
| `uvicorn.run(..., reload=True)` or `--reload` in production | convenient in development | never in production; use a process manager and no reload |
| `Form`/`UploadFile` parameter fails at startup | `python-multipart` not installed | it is in the pinned requirements; install them with `pip install -r requirements.txt` |
| Reading a whole upload with `await file.read()` | it is the shortest call | read in chunks; remember the framework already received the body, so also limit size at the proxy |
| Mutable module-level state (`tasks = []`) shared by tests and requests | it is the easiest store | a `create_app()` factory plus `Depends` and `app.state` |
| Using the client's filename as a path | it looks like a harmless string | keep only the last component, replace odd characters, store under a generated id |

## 5. Coming From Other Languages
- **Node/Express/Fastify:** FastAPI routes look like Fastify's with schema; middleware `call_next` is Express's `next()` but async; there is one event loop, so blocking code hurts the same way.
- **Java/C# (Spring, ASP.NET):** decorators play the role of attributes/annotations; `Depends` is constructor/parameter injection; `lifespan` is `IHostedService`/`@PostConstruct`.
- **Go (`net/http`):** the stdlib example is the closest analogue; FastAPI adds binding and validation that Go does by hand. Handlers are not goroutines: concurrency is the event loop plus a threadpool.
- **Ruby/Rails:** a decorator-based router instead of `routes.rb`; type hints replace strong parameters.

## 6. Exercises
Starter code and tests are in `exercises/`; reference solutions are in `solutions/`. Tests use FastAPI's `TestClient`, so no port is needed.

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | [Routes](exercises/ex1_routes/m02_routes.py) | Easy | path, query, header binding | `pytest modules/02-http-server/exercises/ex1_routes` |
| 2 | [Middleware](exercises/ex2_middleware/m02_middleware.py) | Medium | request id, recovery, CORS, order | `pytest modules/02-http-server/exercises/ex2_middleware` |
| 3 | [Streaming files](exercises/ex3_streaming/m02_files.py) | Stretch | multipart, chunked I/O, ETag/304 | `pytest modules/02-http-server/exercises/ex3_streaming` |

**Exercise 1: Routes**
- **Task:** implement `create_app()` with the six routes listed in its docstring.
- **Acceptance:** all tests in `test_m02_routes.py` pass, including `422` for bad input and a repeated `tag` query parameter.
- <details><summary>Hint</summary>Stack two <code>@app.get</code> decorators to share one handler between <code>/hello</code> and <code>/hello/{name}</code>; <code>Annotated[int, Path(ge=1)]</code> constrains path values.</details>

**Exercise 2: Middleware**
- **Task:** implement `install_middleware(app)`: request id (validated, generated if invalid), timing header, recovery to a `500` envelope without leaking details, and CORS for one origin.
- **Acceptance:** all tests in `test_m02_middleware.py` pass, including CORS headers on the `500` response.
- <details><summary>Hint</summary>Wrap <code>await call_next(request)</code> in <code>try/except Exception</code> inside the http middleware, then add <code>CORSMiddleware</code> afterwards so it is the outermost layer.</details>

**Exercise 3: Streaming files**
- **Task:** implement `sanitize_filename` and `create_app(storage_dir, max_bytes)`: chunked upload with hashing and a size limit, plus a streamed, cacheable download.
- **Acceptance:** all tests in `test_m02_files.py` pass: round trip over several chunks, `413` leaves no file, hostile names stay inside the storage directory, `If-None-Match` gives `304`.
- <details><summary>Hint</summary>A plain <code>def</code> upload handler can use <code>file.file.read(CHUNK_SIZE)</code>; write to a temporary file first and <code>Path.replace</code> it to its final name; always <code>unlink(missing_ok=True)</code> the temporary file in <code>finally</code>.</details>

## 7. Recap
- ASGI server (uvicorn) + framework (FastAPI): the framework turns type hints into binding and validation.
- Plain `def` handlers are for blocking work, `async def` handlers must never block.
- Middleware wraps handlers; the last one added is the outermost. Put recovery inside, CORS outside.
- Use `Depends` for shared objects and `lifespan` for startup/shutdown; use a `create_app()` factory.
- Graceful shutdown (SIGTERM, drain in-flight requests) and timeouts are part of the server, not an afterthought.

When to reach for what: simple JSON service -> FastAPI; need only a health endpoint with no dependencies -> `http.server` is acceptable; streaming large bodies -> `StreamingResponse` with a generator.

## 8. Self-check Questions
1. What happens to other requests if an `async def` handler runs `time.sleep(5)`? What if it is a plain `def`?
2. In what order do two middlewares run on the request and on the response, and why does that matter for CORS on error responses?
3. How does FastAPI decide whether a parameter comes from the path, the query string, the body or a header?
4. What exactly does graceful shutdown guarantee, and what does `timeout_graceful_shutdown` do when a client never finishes?
5. Why is limiting upload size inside the handler not enough on its own?

## 9. Further Reading
- [FastAPI user guide](https://fastapi.tiangolo.com/tutorial/): path/query parameters, middleware, request files
- [Starlette](https://www.starlette.io/) documentation (middleware, responses, static files) and the [ASGI specification](https://asgi.readthedocs.io/)
- [uvicorn settings](https://www.uvicorn.org/settings/) (timeouts, graceful shutdown)

---
**Next:** [03 REST API Design](../03-rest-api-design/README.md)
