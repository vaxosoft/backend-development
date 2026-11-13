# 09. Configuration and Environment in Python

> Estimated time: 1.5-2h | Difficulty: Intermediate | Prerequisites: 01 (language fundamentals, dataclasses), 02 (HTTP server, for the wiring example), 04 (serialization, for TOML and pydantic)

## Learning Outcomes
After this module you can:
- [ ] Read environment variables and parse their string values into typed values
- [ ] Combine defaults, a config file, environment variables and command-line flags with a clear precedence
- [ ] Validate configuration at startup and report every problem at once
- [ ] Keep secrets out of logs, reprs and version control
- [ ] Build typed settings with `pydantic-settings`
- [ ] Wire an application by hand from its settings (composition root) and override pieces in tests

## 1. Concept Overview
Configuration is everything that changes between environments but not between releases: ports, URLs, log levels, credentials. The twelve-factor rule is to take it from the **environment**, with defaults in code and optionally a file for local development. When the same key appears in several places, a fixed precedence decides: **defaults < config file < environment < command line**.

Three habits matter more than the mechanism. **Parse once, early:** turn raw strings into a typed object at startup and pass that object around (no `os.environ` reads scattered through the code). **Fail fast:** an invalid or missing value should stop the process with a clear message, not surface later as a 500. **Treat secrets differently:** they come from the environment or a secret store, never from the repository, and they must not appear in logs.

## 2. How Python Approaches It

| Tool | Use it for |
|------|-----------|
| `os.environ` (a `dict[str, str]`) | raw access; `.get(name, default)` for optional, `[name]` (KeyError) for required |
| `argparse` | command-line flags; pass an explicit list for tests; `exit_on_error=False` raises instead of exiting |
| `tomllib` (stdlib, read-only) | config files; `load()` needs a *binary* file object |
| `collections.ChainMap` | layered lookup: the first mapping that has the key wins |
| `dataclass(frozen=True)` | a stdlib typed config object with validation in `__post_init__` |
| `pydantic-settings` `BaseSettings` | typed settings from env/`.env`/TOML with validation, `SecretStr`, and a customisable source order |
| `Depends`, `app.state` | FastAPI wiring and per-test overrides |

Pydantic-settings is the ecosystem choice ([PROFILE](../../PROFILE.md)) because annotations do the parsing and validation; the stdlib version is shown first so you know what it does for you. Dependency injection in Python rarely needs a framework: build the object graph in one function (the *composition root*) and pass dependencies as constructor or function arguments.

## 3. Guided Examples
Run from `languages/python/`. The examples set up their own environment with `unittest.mock.patch.dict`, so your shell variables do not matter.

### 3.1 Environment basics
- **Goal:** env values are strings; parse them explicitly.
- **Run:** `python modules/09-configuration/examples/01_environ_basics.py`
- **Code:** [01_environ_basics.py](examples/01_environ_basics.py)

```python
bool(os.environ["TT_DEBUG"])  # True, even for "false": any non-empty string is truthy
env_bool("TT_DEBUG")  # False: parsed against known words
```

- **Walkthrough:** `env_bool`, `env_int` and `env_required` read `os.environ` and raise a `ConfigError` that names the variable. `os.environ.get` returns `None` for a missing name; `os.environ[name]` raises `KeyError`, which `env_required` turns into a readable message.
- **Expected output:**

  ```
  bool('false') is True
  env_bool('TT_DEBUG') is False
  port: 9090 | missing with default: 2
  os.environ.get for a missing name: None
  home: /srv/tasks
  ConfigError: TT_API_TOKEN is required
  ConfigError: TT_HOME: expected an integer, got '/srv/tasks'
  ```

### 3.2 Flags and files: argparse and tomllib
- **Goal:** the two other input channels.
- **Run:** `python modules/09-configuration/examples/02_argparse_and_toml.py`
- **Code:** [02_argparse_and_toml.py](examples/02_argparse_and_toml.py)

```python
parser.add_argument("--port", type=int, default=None)  # None = "not given"
parser.add_argument("--debug", action=argparse.BooleanOptionalAction, default=None)
with path.open("rb") as handle:  # tomllib wants bytes
    tomllib.load(handle)
```

- **Walkthrough:** defaulting every flag to `None` is what lets a lower layer decide later. By default argparse calls `sys.exit` on a bad flag, which is hostile to tests and libraries; `exit_on_error=False` raises `ArgumentError`. TOML gives real types (ints, lists, nested tables).
- **Expected output:**

  ```
  Namespace(host=None, port=9000, debug=True)
  Namespace(host=None, port=None, debug=None)
  ArgumentError: argument --port: invalid int value: 'x'
  ArgumentError: unrecognized arguments: --unknown
  {'host': '0.0.0.0', 'port': 8080} ['stderr', 'file']
  Task Tracker
  TOMLDecodeError: Invalid value (at end of document)
  ```

### 3.3 Layered configuration by hand
- **Goal:** explicit precedence and validation with the standard library only.
- **Run:** `python modules/09-configuration/examples/03_layered_config.py`
- **Code:** [03_layered_config.py](examples/03_layered_config.py)

```python
merged = ChainMap(from_cli(argv), from_env(env), tomllib.loads(file_text), DEFAULTS)
Config(str(merged["host"]), int(str(merged["port"])), str(merged["log_level"]))
```

- **Walkthrough:** each layer is a plain dict containing only the keys it really has (`from_cli` drops the `None`s). `ChainMap` searches them left to right. `Config.__post_init__` validates, so an invalid value can never become a `Config`. The sources are parameters, so tests need no real environment.
- **Expected output:**

  ```
  defaults only:      Config(host='127.0.0.1', port=8080, log_level='info')
  + file:             Config(host='127.0.0.1', port=9000, log_level='warn')
  + file + env:       Config(host='0.0.0.0', port=9100, log_level='warn')
  + file + env + cli: Config(host='0.0.0.0', port=9200, log_level='debug')
  port=0: ConfigError: port must be 1-65535, got 0
  port=abc: ConfigError: port must be an integer: invalid literal for int() with base 10: 'abc'
  bad level: ConfigError: log_level must be one of ('debug', 'info', 'warn', 'error'), got 'loud'
  ```

### 3.4 Typed settings with pydantic-settings
- **Goal:** let annotations parse and validate; hide secrets; reorder sources.
- **Run:** `python modules/09-configuration/examples/04_pydantic_settings.py`
- **Code:** [04_pydantic_settings.py](examples/04_pydantic_settings.py), model in [m09_task_settings.py](examples/m09_task_settings.py)

```python
class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="TT_")
    port: int = Field(8080, ge=1, le=65535)
    log_level: Literal["debug", "info", "warn", "error"] = "info"
    api_token: SecretStr  # no default: required

    @classmethod
    def settings_customise_sources(
        cls, settings_cls, init_settings, env_settings, dotenv_settings, file_secret_settings
    ):
        return (
            init_settings,
            env_settings,
            dotenv_settings,
            TomlConfigSettingsSource(settings_cls),
        )
```

- **Walkthrough:** `TT_PORT=9000` becomes `int`, `TT_DEBUG=yes` becomes `True`. `SecretStr` masks itself in `repr` and JSON; you must call `get_secret_value()` on purpose. Validation reports all failures together. The tuple returned by `settings_customise_sources` is the precedence (first wins); the subclass in the example only supplies `toml_file`.
- **Expected output:**

  ```
  port=9000 log_level=debug debug=True database_url=sqlite:///tasks.db
  repr hides the secret: SecretStr('**********')
  explicit reveal: s3cr3t
  TT_PORT: Input should be less than or equal to 65535
  TT_LOG_LEVEL: Input should be 'debug', 'info', 'warn' or 'error'
  TT_API_TOKEN: Field required
  port=7100 log_level=error debug=True database_url=sqlite:///from-toml.db
  port=7300 log_level=error debug=True database_url=sqlite:///from-toml.db
  ```

  In the last two lines `port` comes from `.env` (7100) and then from the init argument (7300), `log_level` from the environment, `debug` from `.env` and `database_url` from the TOML file.

### 3.5 Task Tracker wiring (composition root)
- **Goal:** build the app from one `Settings` object, with no global state.
- **Run:** `python modules/09-configuration/examples/05_task_tracker_wiring.py`
- **Code:** [05_task_tracker_wiring.py](examples/05_task_tracker_wiring.py)

```python
def create_app(settings: Settings) -> FastAPI:
    app = FastAPI(title="Task Tracker", debug=settings.debug)
    app.state.settings = settings
    app.state.repo = TaskRepository()
    ...  # routes depend on get_settings / get_repo
```

- **Walkthrough:** `create_app` is the only place that decides what is wired to what. Handlers receive settings and the repository through `Depends`, so a second app with different settings can live in the same process (the example runs two, through `httpx.ASGITransport`, no socket). `/config` returns the settings with the secret masked; `POST /tasks` checks the token with `secrets.compare_digest`. In larger apps FastAPI's `app.dependency_overrides` replaces a dependency per test.
- **Expected output:**

  ```
  {'port': 8080, 'log_level': 'warn', 'database_url': 'sqlite:///tasks.db', 'api_token': '**********', 'debug': False}
  401
  201 {'id': 1, 'title': 'Write docs'}
  True
  production token on the testing app: 401
  ```

## 4. Common Pitfalls

| Mistake | Why it happens | Fix |
|---------|----------------|-----|
| `if os.environ.get("DEBUG"):` | any non-empty string, including `"false"` and `"0"`, is truthy | parse against known words, or use a `bool` field in pydantic-settings |
| `int(os.environ["PORT"])` at the call site | scattered reads, `KeyError`/`ValueError` far from startup | read and validate once into a typed settings object at startup |
| Reading `os.environ` at import time | module-level constants look tidy | tests and CLIs cannot change them afterwards; build settings in a function |
| `argparse` calling `sys.exit` | its default error handling | `exit_on_error=False` (and default flags to `None`) when you need to layer or test |
| Using `argparse` defaults as the config defaults | the flag default hides whether the user passed it | default to `None`, merge with the other layers, then apply defaults |
| `SecretStr` printed via `get_secret_value()` into logs | "just for debugging" | log `settings.model_dump(mode="json")` instead; reveal secrets only at the point of use |
| Committing `.env` | it is convenient for local runs | add `.env` to `.gitignore`, commit `.env.example` with placeholders |
| `list[str]` field from an env var fails | pydantic-settings decodes complex types as JSON | mark the field `Annotated[list[str], NoDecode]` and split commas in a `field_validator(mode="before")` |
| Silent typos (`TT_PROT=1`) | unknown names are ignored | for files, reject unknown keys; log the effective config at startup |
| Module-level `settings = Settings()` shared by everything | global singleton | create it in `main`/`create_app` and pass it down (tests construct their own) |

## 5. Coming From Other Languages
- **JavaScript/TypeScript:** `process.env` is also all strings; `dotenv` plus `zod` is the equivalent of `.env` plus `BaseSettings`.
- **Java/C#:** `BaseSettings` plays the role of Spring `@ConfigurationProperties` / `IOptions<T>`; the `settings_customise_sources` tuple is the provider order of `ConfigurationBuilder`. Python needs no DI container: the composition root is a function.
- **Go:** `os.Getenv` returns `""` for missing, like `.get(name, "")`; struct tags with `envconfig` map to pydantic fields. Errors are exceptions, not returned values.
- **Ruby:** `ENV.fetch("X")` (raises) versus `ENV["X"]` (nil) is exactly `os.environ["X"]` versus `.get`.

## 6. Exercises
Starters and tests are in [exercises/](exercises/); reference solutions in [solutions/](solutions/). Tests never read your real environment.

| # | Title | Difficulty | Skill practiced | Run tests |
|---|-------|-----------|-----------------|-----------|
| 1 | [Parse env values](exercises/ex1_parse_env/m09_env_parsing.py) | Easy | strings to typed values, clear errors | `pytest modules/09-configuration/exercises/ex1_parse_env` |
| 2 | [Layered config](exercises/ex2_layered_config/m09_layered.py) | Medium | precedence, validate-all, argparse without `sys.exit` | `pytest modules/09-configuration/exercises/ex2_layered_config` |
| 3 | [Typed settings](exercises/ex3_settings/m09_settings.py) | Stretch | pydantic-settings, `SecretStr`, comma lists, source order | `pytest modules/09-configuration/exercises/ex3_settings` |

**Exercise 1: parsing helpers.** *Task:* implement `require`, `parse_bool`, `parse_int` (with bounds) and `parse_csv`; every failure is a `ConfigError` naming the variable. *Acceptance:* all tests in `test_m09_env_parsing.py`.
<details><summary>Hint</summary>Catch the <code>ValueError</code> from <code>int()</code> and raise your own error <code>from None</code>; normalise with <code>.strip().lower()</code> before comparing words.</details>

**Exercise 2: `load_config`.** *Task:* merge defaults, file values, `APP_*` environment variables and CLI flags; validate only the winning values; raise one `ConfigError` listing every problem. *Acceptance:* all tests in `test_m09_layered.py`.
<details><summary>Hint</summary>Build one dict per layer containing only the keys that layer set, put them in a <code>ChainMap</code>, then convert each key with a small function that raises <code>ValueError</code>.</details>

**Exercise 3: `UploadSettings`.** *Task:* add the field constraints, the secret, the comma-separated list parsing and the TOML source so that the layering tests pass. *Acceptance:* all tests in `test_m09_settings.py`.
<details><summary>Hint</summary><code>Field(ge=, le=)</code> and <code>Field(min_length=)</code> on the fields; <code>NoDecode</code> plus a <code>field_validator(mode="before")</code> for the list; return the TOML source last from <code>settings_customise_sources</code>.</details>

## 7. Recap
- Parse strings into typed values once, at startup, into one immutable object.
- Precedence is defaults < file < environment < command line; implement it explicitly or let `BaseSettings` do it.
- Fail fast and report all problems together.
- Secrets live in the environment, are wrapped (`SecretStr`), and never reach logs or Git.
- Wire the app in one function and pass dependencies down; tests build their own settings.

## 8. Self-check Questions
1. Why does `bool("false")` return `True`, and what does a typed settings class do differently?
2. A bad `port` is set in the config file but the environment overrides it with a valid one. Should startup fail? Why does the order of validation and merging matter?
3. Why are argparse defaults problematic for layered configuration?
4. What is the difference between a module-level `settings = Settings()` and creating settings in `create_app`, for testing?

## 9. Further Reading
- [pydantic-settings documentation](https://docs.pydantic.dev/latest/concepts/pydantic_settings/)
- [argparse](https://docs.python.org/3/library/argparse.html), [tomllib](https://docs.python.org/3/library/tomllib.html), [collections.ChainMap](https://docs.python.org/3/library/collections.html#collections.ChainMap)
- [The Twelve-Factor App: Config](https://12factor.net/config)

---
**Next:** [10 Logging and Diagnostics](../10-logging-and-diagnostics/README.md)
