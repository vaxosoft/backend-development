# P7: Tested Service with Docker and CI

**Modules exercised:** [09 Configuration](../../modules/09-configuration/README.md), [10 Logging and diagnostics](../../modules/10-logging-and-diagnostics/README.md), [12 Testing](../../modules/12-testing/README.md), [14 Packaging and running](../../modules/14-packaging-and-running/README.md).

## Goal

Take a small Tasks service and make it **operable**: readiness and metrics endpoints, configuration that fails fast with a clear message, a multi-stage non-root `Dockerfile`, a `.dockerignore` and a GitHub Actions workflow. The tests check the behaviour and also read the packaging files and assert the rules. Docker itself is never run.

## Requirements

Already in the starter (`starter/p7_tested_service_docker_ci/`): `POST /tasks` (`201` + `Location`), `GET /tasks`, `GET /tasks/{id}`, `GET /healthz`, the shared error envelope, and an in-memory store behind the `TaskStore` protocol.

| Piece | Behaviour |
|-------|-----------|
| `GET /readyz` | `503` `{"status":"not_ready"}` until the application has started, then `200` `{"status":"ready"}` (flip a flag in a lifespan handler; a client that never ran the lifespan sees `503`) |
| `GET /metrics` | `200` `text/plain`, one line `requests_total <n>`. `n` counts **every** request received (404s and the scrape itself included), so the first scrape of a fresh app shows `requests_total 1` |
| `load_config(env)` | pure function over a mapping. `HOST` (default `127.0.0.1`, not blank), `PORT` (integer 1-65535, default `8080`), `LOG_LEVEL` (`debug`, `info`, `warn`, `error`, case-insensitive, default `info`), `ENV` (`dev`, `test`, `prod`, default `dev`). Any invalid value raises `ConfigError` whose message names **every** offending variable and its value, e.g. `invalid configuration: PORT must be an integer from 1 to 65535, got 'abc'; LOG_LEVEL must be one of debug, info, warn, error, got 'loud'` |
| `starter/Dockerfile` | at least two `FROM` stages (the first named with `AS`), base images pinned to a version tag (not `latest`), dependencies installed in the build stage and copied with `COPY --from=`, **no** `pip install`/`apt-get` in the final stage, final stage runs as a non-root `USER`, has `EXPOSE`, `HEALTHCHECK` and `CMD` |
| `starter/.dockerignore` | contains at least `.git`, `.venv`, `.env` and `__pycache__/` |
| `starter/.github/workflows/ci.yml` | triggers on `push` and `pull_request`; steps install dependencies, `ruff format --check`, `ruff check` and `pytest` in that order; `actions/setup-python` uses the same minor version as the image's `python:<version>` tag |

The service entry point (`python -m p7_tested_service_docker_ci`) calls `load_config(os.environ)`, prints `error: ...` and exits with status 2 when it raises, and otherwise serves on `HOST:PORT`. `ADDR=host:port` overrides both, like in the other lessons.

## Run

```bash
python -m pytest projects/P7-tested-service-docker-ci/tests      # tests (from languages/python)
cd projects/P7-tested-service-docker-ci/starter
ADDR=127.0.0.1:8080 LOG_LEVEL=debug python -m p7_tested_service_docker_ci
PORT=abc python -m p7_tested_service_docker_ci                   # error: invalid configuration: ... (exit 2)
```

```bash
curl -i localhost:8080/readyz                                   # 200 {"status":"ready"} once started
curl -X POST localhost:8080/tasks -H 'content-type: application/json' -d '{"title":"Write docs","tags":["docs"]}'
curl localhost:8080/tasks/1
curl localhost:8080/metrics                                     # requests_total 4 after the calls above
```

The solution adds a `Dockerfile` you can build where Docker is available (from `starter/`): `docker build -t tasks-service .` and `docker run --rm -p 8080:8080 tasks-service`. The container listens on `0.0.0.0:8080` because the image sets `HOST` and `PORT`.

## Acceptance criteria (tests)

| Criterion | Tests |
|-----------|-------|
| health, tasks (given) | `test_healthz_reports_ok`, `test_create_task_returns_201_location_and_defaults`, `test_get_task_returns_the_stored_task`, `test_unknown_task_is_404_with_error_envelope`, `test_list_tasks_returns_tasks_in_creation_order`, `test_invalid_task_is_422_with_field_messages`, `test_app_talks_to_the_store_interface_so_a_fake_can_be_injected` (`tests/test_p7_behaviour.py`) |
| readiness | `test_readyz_is_503_until_startup_has_finished`, `test_readyz_is_200_once_the_app_has_started` |
| metrics | `test_metrics_is_plain_text_with_requests_total_line`, `test_metrics_counts_every_request_including_the_scrape_itself` |
| configuration | `test_empty_environment_gives_the_defaults`, `test_valid_values_are_parsed_and_log_level_is_case_insensitive`, `test_invalid_value_fails_with_a_message_naming_the_variable`, `test_every_problem_is_reported_at_once` (`tests/test_p7_config.py`) |
| Dockerfile | `test_dockerfile_is_multi_stage_with_a_named_build_stage`, `test_final_stage_copies_the_build_output_and_does_not_install_anything`, `test_final_stage_runs_as_a_non_root_user`, `test_dockerfile_exposes_a_port_and_declares_a_healthcheck_and_a_command`, `test_base_images_are_pinned_to_a_version_tag` (`tests/test_p7_hygiene.py`) |
| `.dockerignore` | `test_dockerignore_keeps_secrets_and_junk_out_of_the_build_context` |
| CI | `test_ci_runs_on_push_and_pull_request`, `test_ci_checks_formatting_lints_and_tests_in_that_order`, `test_ci_uses_the_same_python_version_as_the_image` |

The behaviour tests use `fastapi.testclient.TestClient` (use it as a context manager to run the lifespan), a clock injected into `InMemoryTaskStore`, and a hand-written fake `TaskStore`. `load_config` is tested with plain dicts, never with the real environment.

## What to implement

| File | Work |
|------|------|
| `starter/p7_tested_service_docker_ci/config.py` | `load_config` |
| `starter/p7_tested_service_docker_ci/app.py` | the `TODO(readiness)` and `TODO(metrics)` blocks: lifespan flag, request-counting middleware, `/readyz`, `/metrics` |
| `starter/requirements.txt`, `starter/requirements-dev.txt` | pinned runtime and tooling dependencies |
| `starter/Dockerfile`, `starter/.dockerignore` | the container build |
| `starter/.github/workflows/ci.yml` | the pipeline |

The workflow is written for a standalone repository whose root is the service folder (`tests/` next to the package, so `python -m pytest` finds them). In this course the tests live beside `starter/`; copy the folder out or adapt the paths when you use it for real.

<details>
<summary>Hints</summary>

- Collect problems in a list and raise once at the end; `int()` raises `ValueError` for non-numbers and `" 80 "` is accepted, so check the range yourself.
- `FastAPI(lifespan=...)` takes an async context manager; code before `yield` runs at startup, after it at shutdown. `TestClient(app)` runs it only inside a `with` block.
- `@app.middleware("http")` receives `(request, call_next)`; increment the counter before `await call_next(request)`. Return `PlainTextResponse` (or `response_class=PlainTextResponse`) for `/metrics`.
- Build stage: `python -m venv /opt/venv`, `pip install -r requirements.txt` with `requirements.txt` copied **before** the source so the layer is cached. Final stage: `COPY --from=build /opt/venv /opt/venv`, put it on `PATH`, `useradd` a system user and switch with `USER`.
- `PYTHONUNBUFFERED=1` makes logs appear immediately; `HEALTHCHECK CMD python -c "import urllib.request as u; u.urlopen('http://127.0.0.1:8080/healthz', timeout=2)"` needs no `curl` in the slim image.
- In GitHub workflows the key `on:` is parsed as boolean `True` by YAML 1.1 loaders such as PyYAML; the hygiene test already handles it.

</details>

## Stretch goals (not tested)

- Add a `/metrics` histogram of request durations (`requests_duration_seconds_bucket`) by hand.
- Make `/readyz` depend on a real dependency check on the store.
- Use `pydantic-settings` for the configuration and compare the error messages.
- Pin base images by digest and add a `docker/build-push-action` job that pushes on tags.
- Add a coverage gate (`pytest --cov --cov-fail-under=90`) to the workflow.

## Reference solution

[`solution/`](./solution/) holds only the files that differ from the starter, same relative paths: `config.py`, `app.py`, `requirements.txt`, `requirements-dev.txt`, `Dockerfile`, `.dockerignore` and `.github/workflows/ci.yml`. Overlay it onto `starter/` (include the hidden files) to run the whole suite green.
