# 14. Packaging and Running in Python

> Estimated time: 1.5-2 h | Difficulty: Intermediate | Prerequisites: 02 (HTTP server), 09 (configuration), 10 (logging)

## Learning Outcomes
After this module you can:
- [ ] Describe a Python service in `pyproject.toml` and install it into a virtual environment, never the system Python
- [ ] Produce a distributable artifact (wheel or `zipapp`) and know what each one contains
- [ ] Start a service with explicit runtime flags, production-safe uvicorn settings and clean shutdown on SIGTERM
- [ ] Write a multi-stage Dockerfile (slim image, cached dependency layer, non-root user, health check)
- [ ] Read a basic CI workflow: install, format check, lint, type check, test, image build

## 1. Concept Overview
"Packaging" answers: what exactly do we ship, and how does it start in a place we do not control? For a service that means a pinned set of dependencies, the application code, a version that can be read at runtime, an entry point with a few documented flags, and a process that stops cleanly when the orchestrator says so. Containers and CI make this repeatable: the same image runs on a laptop and in production, and every push proves that it still builds, passes the checks and starts.

```text
pyproject.toml + code --build--> wheel / zipapp / image --run--> process (flags + env)
                                                                    |
CI: install -> format -> lint -> types -> test -> image  <---- same commands as locally
```

## 2. How Python Approaches It
There is no compiled binary and no cross-compilation: the artifact is source (or bytecode) plus an interpreter plus dependencies. Portability comes from pinning the interpreter version and the dependency set.

| Need | Tool | Note |
|------|------|------|
| Project metadata, dependencies, entry points | `pyproject.toml` (`[project]`, `[build-system]`) | the one config file; read it with `tomllib` |
| Isolated dependencies | `python -m venv`, `pip install -r requirements.txt` | pinned with `==`; one venv per project, created inside the image rather than copied from your machine |
| Distributable archive | `pip wheel . --no-deps` (needs a build backend such as setuptools), `python -m zipapp` | a wheel is a zip with the code and a `.dist-info` folder; a zipapp is a runnable zip |
| Version at runtime | `importlib.metadata.version("dist-name")`, or `APP_VERSION`/`GIT_COMMIT` injected at build time | the second works without installing the project |
| Running | `uvicorn module:app --host --port --workers --log-level` | `--reload` is for development only |
| Container | multi-stage Dockerfile on `python:3.14-slim` | build stage makes a venv, runtime stage copies it |
| CI | GitHub Actions workflow | the same `ruff`, `mypy`, `pytest` commands |

Idiomatic: pinned requirements, `python -m venv` in a build stage, the environment (`ADDR`, `APP_ENV`, ...) for deployment settings, flags for per-run overrides, exec-form `CMD`. Non-idiomatic: `pip install` into the system interpreter, `sudo pip`, `--reload` in production, `CMD python app.py` (shell form), `latest` tags. Third-party packages here are only `fastapi` and `uvicorn`; publishing to PyPI is out of scope.

## 3. Guided Examples

### 3.1 Project metadata
- **Goal:** read what `pyproject.toml` declares and what is actually installed.
- **Run:** `python modules/14-packaging-and-running/examples/01_project_metadata.py`
- **Code:** [examples/01_project_metadata.py](./examples/01_project_metadata.py) reads [examples/sample_pyproject.toml](./examples/sample_pyproject.toml)
- **Walkthrough:** `tomllib` (stdlib, read-only) parses the sample. `[project.scripts]` maps a command (`task-tracker`) to a function that `pip install` turns into an executable. `importlib.metadata.version` answers for installed distributions, which here are pinned in [../../requirements.txt](../../requirements.txt).
- **Expected output:**

  ```text
  name: task-tracker
  version: 1.4.2
  requires-python: >=3.14
  dependencies: ['fastapi>=0.162,<1', 'uvicorn>=0.54,<1']
  extras: ['dev']
  entry points: {'task-tracker': 'task_tracker.__main__:main'}
  build backend: setuptools.build_meta
  installed uvicorn: 0.54.0
  python >= 3.14: True
  running under -O: False
  ```

### 3.2 Runtime flags
- **Goal:** flags override environment, environment overrides defaults, invalid combinations fail fast.
- **Run:** `python modules/14-packaging-and-running/examples/02_runtime_flags.py --addr 0.0.0.0:9000 --workers 4`
- **Code:** [examples/02_runtime_flags.py](./examples/02_runtime_flags.py)

  ```python
  parser.add_argument("--addr", default=environ.get("ADDR", "127.0.0.1:8080"))
  ...
  if args.reload and args.workers > 1:
      print("error: --reload cannot be combined with --workers > 1", file=sys.stderr)
      return 2
  ```

- **Walkthrough:** `argparse` reads `sys.argv`; defaults come from the environment, so an explicit flag wins. `choices=` rejects unknown log levels with exit status 2. The program prints the uvicorn command it would run: in production `uvicorn task_tracker.app:app --workers N` starts several processes (workers need an import string, not an app object).
- **Expected output:**

  ```text
  $ python modules/14-packaging-and-running/examples/02_runtime_flags.py --addr 0.0.0.0:9000 --workers 4
  resolved: {'addr': '0.0.0.0:9000', 'workers': 4, 'log_level': 'info', 'reload': False}
  command: uvicorn task_tracker.app:app --host 0.0.0.0 --port 9000 --workers 4 --log-level info
  $ ADDR=0.0.0.0:7000 WORKERS=2 python modules/14-packaging-and-running/examples/02_runtime_flags.py
  resolved: {'addr': '0.0.0.0:7000', 'workers': 2, 'log_level': 'info', 'reload': False}
  command: uvicorn task_tracker.app:app --host 0.0.0.0 --port 7000 --workers 2 --log-level info
  $ python modules/14-packaging-and-running/examples/02_runtime_flags.py --reload --workers 2
  error: --reload cannot be combined with --workers > 1
  ```

### 3.3 A self-contained archive with `zipapp`
- **Goal:** see that an "artifact" is just a zip with an entry point.
- **Run:** `python modules/14-packaging-and-running/examples/03_zipapp.py`
- **Code:** [examples/03_zipapp.py](./examples/03_zipapp.py)

  ```python
  zipapp.create_archive(source, archive, interpreter="/usr/bin/env python3", main="cli:main")
  ```

- **Walkthrough:** the example writes two modules into a temporary folder, packs them (zipapp adds a `__main__.py` that calls `cli:main` and a shebang line), and runs the archive with the current interpreter. `zipapp` bundles only pure-Python code: dependencies must be installed into the folder first (`pip install --target app/ -r requirements.txt`), and compiled extensions do not work from inside a zip. For a wheel the equivalent command is `pip wheel . --no-deps`, which needs the build backend from `[build-system]` and therefore network access; it produces `task_tracker-1.4.2-py3-none-any.whl`.
- **Expected output:**

  ```text
  archive: greeter.pyz | shebang: #!/usr/bin/env python3
  entries: ['__main__.py', 'cli.py', 'greeter.py']
  run: Hello, world!
  run: Hello, Task Tracker!
  ```

### 3.4 Task Tracker service entry point
- **Goal:** a production-minded `main()`: settings from the environment, health and readiness endpoints, build info, graceful shutdown.
- **Run:** `APP_VERSION=1.4.2 GIT_COMMIT=abc1234 python modules/14-packaging-and-running/examples/04_task_service.py` (honours `ADDR`, default `127.0.0.1:8080`; stop with Ctrl-C or `kill <pid>`)
- **Code:** [examples/04_task_service.py](./examples/04_task_service.py)

  ```python
  config = uvicorn.Config(
      create_app(),
      host=host,
      port=port,
      timeout_graceful_shutdown=int(os.environ.get("SHUTDOWN_TIMEOUT", "10")),
      server_header=False,
  )
  signal.signal(signal.SIGTERM, signal.SIG_IGN)  # see walkthrough
  with suppress(KeyboardInterrupt):
      uvicorn.Server(config).run()
  ```

- **Walkthrough:** `/healthz` (liveness) always answers; `/readyz` flips to 200 once the lifespan startup has run and back to 503 on shutdown. uvicorn handles SIGINT/SIGTERM by finishing in-flight requests (up to `timeout_graceful_shutdown`), then *re-raises the signal*, so a plain run ends with status 143 or a `KeyboardInterrupt` traceback; ignoring SIGTERM beforehand and suppressing `KeyboardInterrupt` makes the exit status 0. `Server(config).run()` serves a single process; use `uvicorn module:app --workers N` for several.
- **Expected output:**

  ```text
  $ curl -s localhost:8080/healthz
  {"status":"ok"}
  $ curl -s localhost:8080/readyz
  {"status":"ready"}
  $ curl -s localhost:8080/version
  {"version":"1.4.2","commit":"abc1234","env":"development"}
  $ curl -s localhost:8080/tasks
  {"items":[{"id":1,"title":"Write docs","status":"todo","tags":["docs"]},{"id":2,"title":"Ship v1","status":"in_progress","tags":[]}],"total":2,"limit":20,"offset":0}
  ```

### 3.5 Dockerfile, `.dockerignore` and CI for the service (written, not run)
- **Goal:** read the packaging files for the service above. Docker is not required for this course, so these files are checked by the linter you build in exercise 3 rather than built.
- **Run:** `docker build -f modules/14-packaging-and-running/examples/deploy/Dockerfile -t task-tracker modules/14-packaging-and-running/examples` from `languages/python/` (the last argument is the build context), then `docker run --rm -p 8080:8080 task-tracker`
- **Code:** [examples/deploy/Dockerfile](./examples/deploy/Dockerfile), [examples/.dockerignore](./examples/.dockerignore), [examples/deploy/requirements.txt](./examples/deploy/requirements.txt), [examples/deploy/ci.yml](./examples/deploy/ci.yml)
- **Walkthrough:** the build stage creates `/opt/venv` and installs `requirements.txt` *before* copying code, so editing code reuses the cached dependency layer. The runtime stage starts from a fresh slim image, copies only the venv and the service, runs as a numeric non-root `USER`, sets `PYTHONUNBUFFERED=1` (logs appear immediately) and uses an exec-form `CMD` so Python is PID 1 and receives SIGTERM. `HEALTHCHECK` calls `/healthz` with the stdlib (no `curl` in slim images). Build metadata arrives through `--build-arg`. [ci.yml](./examples/deploy/ci.yml) belongs in `.github/workflows/` of your repository (adjust `working-directory` and the `tests` path): it triggers on push and pull requests, pins Python 3.14, and runs the same format, lint, type and test commands you run locally before building the image.
- **Expected output:** none (not executed here).

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `pip install` into the system Python | Works on a laptop, breaks OS tools (PEP 668 "externally managed" errors) | `python -m venv`, activate it, `pip install -r requirements.txt` |
| `COPY . .` before `pip install` | Any source edit invalidates the dependency layer, so every build reinstalls everything | Copy `requirements.txt`, install, then copy the code |
| `uvicorn --reload` in production | Meant for development: file watcher, one process, restarts on change | no `--reload` outside development; scale with `--workers` or replicas |
| `CMD python service.py` (shell form) | The shell becomes PID 1 and swallows SIGTERM; the container is killed after 10 s | exec form `CMD ["python", "service.py"]` |
| Running as root, `python:latest` | Default image user is root; `latest` changes under you | `useradd` + `USER 10001`; pin `python:3.14-slim` (or a digest) |
| Missing `PYTHONUNBUFFERED=1` | stdout is block-buffered when not a terminal, so logs lag or vanish on a crash | set it in the image, or use `python -u` |
| Copying `.venv` or `__pycache__` into the image | No `.dockerignore`; virtualenvs hard-code absolute paths and the host's platform | ignore `.venv`, `__pycache__`, `.git`, `.env`; build the venv inside the image |
| `--workers 4` with an app object (`uvicorn.run(app, workers=4)`) | Workers are separate processes and must import the app themselves | pass an import string `"package.module:app"` |
| Treating SIGTERM exit status 143 as a crash | uvicorn re-raises the caught signal after a clean shutdown | accept 143 in the orchestrator, or exit 0 as in example 3.4 |
| Secrets baked into image layers or `ENV` | `ARG`/`ENV` values stay in image history | inject at runtime (env, secret store); keep `.env` out of the context |

## 5. Coming From Other Languages
- **Go/Rust:** there is no single static binary and no cross-compilation; you ship an interpreter plus dependencies. `zipapp` is the closest thing and still needs Python on the host. Platform-specific wheels exist for compiled dependencies, which is why the build and runtime stage must use the same base image family.
- **Java/C#:** `pyproject.toml` ≈ `pom.xml`/`.csproj`; a wheel ≈ a jar/nupkg; a venv ≈ a per-project classpath. There is no JIT warm-up story: scale with worker processes.
- **JavaScript/TypeScript:** `requirements.txt` with `==` pins ≈ a lockfile (but only for what you list: pin transitive packages too, or use a lock tool); `pyproject.toml` ≈ `package.json`; `[project.scripts]` ≈ `bin`.
- **Ruby:** venv ≈ Bundler's per-project gems; `Gemfile.lock` ≈ a fully pinned requirements file.

## 6. Exercises
Starter code and tests are in [exercises/](./exercises/); reference solutions in [solutions/](./solutions/). Run them from `languages/python/`.

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | [Build info](./exercises/ex1_build_info/m14_buildinfo.py) | Easy | version from env and `pyproject.toml`, user agent | `pytest modules/14-packaging-and-running/exercises/ex1_build_info` |
| 2 | [Runtime flags](./exercises/ex2_runtime_flags/m14_flags.py) | Medium | precedence, production guards, uvicorn command | `pytest modules/14-packaging-and-running/exercises/ex2_runtime_flags` |
| 3 | [Dockerfile checker](./exercises/ex3_dockerfile_lint/m14_dockerlint.py) | Stretch | multi-stage, non-root, layer caching rules over text | `pytest modules/14-packaging-and-running/exercises/ex3_dockerfile_lint` |

**Exercise 1 - `BuildInfo`, `load_build_info`, `read_project_version`, `parse_version`**
- **Task:** read `APP_VERSION`/`GIT_COMMIT` with defaults, expose a short commit, a user agent and a `/version` payload; read the static version from `pyproject.toml`; parse `MAJOR.MINOR.PATCH`.
- **Acceptance:** defaults `dev`/`unknown`; blank values count as unset; short commit is 7 characters; `dynamic` versions are rejected; `1.10.0 > 1.9.9` after parsing.
- <details><summary>Hint</summary>Use <code>or</code> to fall back for blank strings, <code>tomllib.loads</code> for the project table, and <code>re.fullmatch</code> with three groups.</details>

**Exercise 2 - `parse_runtime_config` and `uvicorn_args`**
- **Task:** resolve address, workers, log level, environment and reload from flags, environment and defaults, and render the uvicorn command.
- **Acceptance:** flag > env > default; production defaults to `min(cpu_count, 4)` workers; invalid ports, levels, environments, abbreviated or unknown flags raise `ConfigError` (never `SystemExit`); `--reload` is refused in production and with several workers.
- <details><summary>Hint</summary>Subclass <code>argparse.ArgumentParser</code> and override <code>error()</code> to raise; pass <code>allow_abbrev=False</code>; validate environment values with the same helpers as flags.</details>

**Exercise 3 - `lint_dockerfile` and `lint_dockerignore`**
- **Task:** parse a Dockerfile (comments, continuation lines, any case) and report rule violations: multi-stage, pinned tags, non-root final user, `--no-cache-dir`, dependency layer order, `EXPOSE`, `HEALTHCHECK`, exec-form `CMD`; check `.dockerignore` entries.
- **Acceptance:** the good Dockerfile and the example in [examples/deploy](./examples/deploy/Dockerfile) yield no findings; each rule fires with the right line number; findings are sorted.
- <details><summary>Hint</summary>Normalise first into <code>(line, KEYWORD, args)</code> tuples, split them into stages at each <code>FROM</code>, then write one small check per rule.</details>

## 7. Recap
- A Python artifact is source plus interpreter plus pinned dependencies; `pyproject.toml` describes it and a venv isolates it.
- Pass deployment settings through the environment and per-run overrides through flags; validate them at startup and refuse unsafe combinations.
- Run uvicorn without `--reload` in production, use exec-form `CMD`, handle SIGTERM and expose liveness/readiness endpoints.
- Dockerfile: multi-stage, dependencies before code, slim base with a pinned tag, non-root user, `PYTHONUNBUFFERED`, `.dockerignore`.
- CI repeats the local commands (format, lint, types, tests) and then builds the image.

## 8. Self-check Questions
1. Why is `uvicorn.run(app, workers=4)` wrong while `uvicorn.run("service:app", workers=4)` works?
2. A container takes 10 seconds to stop. Which Dockerfile line is the likely culprit, and why?
3. What do you lose by pinning only top-level packages in `requirements.txt`, and what are two ways to fix it?
4. Why does the runtime stage copy `/opt/venv` instead of running `pip install` again?
5. Which settings belong in the environment, and which in flags, for a service you run both locally and in an orchestrator?

## 9. Further Reading
- [Python Packaging User Guide](https://packaging.python.org/) and [`pyproject.toml` specification](https://packaging.python.org/en/latest/specifications/pyproject-toml/)
- [uvicorn: settings and deployment](https://www.uvicorn.org/deployment/)
- [Dockerfile best practices](https://docs.docker.com/build/building/best-practices/)
- [zipapp](https://docs.python.org/3/library/zipapp.html) and [venv](https://docs.python.org/3/library/venv.html)
- [GitHub Actions: building and testing Python](https://docs.github.com/en/actions/use-cases-and-examples/building-and-testing/building-and-testing-python)

---
**Next:** back to the [Python language track](../../README.md)
