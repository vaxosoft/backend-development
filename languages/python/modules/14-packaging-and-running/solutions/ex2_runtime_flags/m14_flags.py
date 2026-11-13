"""Runtime configuration for the service process: flags > environment > defaults."""

import argparse
from collections.abc import Mapping, Sequence
from dataclasses import dataclass
from typing import NoReturn

LOG_LEVELS = ("debug", "info", "warning", "error")
ENVIRONMENTS = ("development", "test", "production")


class ConfigError(ValueError):
    """The process was started with invalid flags or environment values."""


@dataclass(slots=True, frozen=True)
class RuntimeConfig:
    host: str
    port: int
    workers: int
    log_level: str
    env: str
    reload: bool


class _Parser(argparse.ArgumentParser):
    def error(self, message: str) -> NoReturn:
        raise ConfigError(message)


def _choice(name: str, value: str, allowed: Sequence[str]) -> str:
    value = value.lower()
    if value not in allowed:
        raise ConfigError(f"{name}: {value!r} is not one of {', '.join(allowed)}")
    return value


def _number(name: str, value: str, minimum: int, maximum: int | None = None) -> int:
    try:
        number = int(value)
    except ValueError:
        raise ConfigError(f"{name}: {value!r} is not an integer") from None
    if number < minimum or (maximum is not None and number > maximum):
        raise ConfigError(f"{name}: {number} is out of range")
    return number


def parse_runtime_config(
    argv: Sequence[str], environ: Mapping[str, str], *, cpu_count: int = 1
) -> RuntimeConfig:
    """Resolve the configuration; raise ConfigError (never SystemExit) on any problem.

    | setting   | flag          | environment | default                                   |
    |-----------|---------------|-------------|-------------------------------------------|
    | address   | --addr        | ADDR        | 127.0.0.1:8080 (empty host = 127.0.0.1)   |
    | workers   | --workers     | WORKERS     | 1; in production min(cpu_count, 4), >= 1  |
    | log level | --log-level   | LOG_LEVEL   | info (debug, info, warning, error; any case) |
    | env       | --env         | APP_ENV     | development (development, test, production) |
    | reload    | --reload      | -           | off                                       |

    Port is 1-65535, workers >= 1. `--reload` is refused in production and with more than one
    worker. Unknown flags and abbreviations such as `--work` are errors.
    """
    parser = _Parser(prog="task-tracker", add_help=False, allow_abbrev=False)
    for flag in ("--addr", "--workers", "--log-level", "--env"):
        parser.add_argument(flag)
    parser.add_argument("--reload", action="store_true")
    args = parser.parse_args(list(argv))

    def pick(flag: str, given: str | None, variable: str) -> tuple[str, str | None]:
        if given is not None:
            return flag, given
        return variable, environ.get(variable)

    name, value = pick("--env", args.env, "APP_ENV")
    env = _choice(name, value if value is not None else "development", ENVIRONMENTS)

    name, value = pick("--log-level", args.log_level, "LOG_LEVEL")
    log_level = _choice(name, value if value is not None else "info", LOG_LEVELS)

    name, value = pick("--addr", args.addr, "ADDR")
    host, _, port_text = (value if value is not None else "127.0.0.1:8080").rpartition(":")
    port = _number(name, port_text, 1, 65535)

    name, value = pick("--workers", args.workers, "WORKERS")
    if value is not None:
        workers = _number(name, value, 1)
    else:
        workers = max(1, min(cpu_count, 4)) if env == "production" else 1

    if args.reload and env == "production":
        raise ConfigError("--reload is not allowed in production")
    if args.reload and workers > 1:
        raise ConfigError("--reload cannot be combined with more than one worker")

    return RuntimeConfig(host or "127.0.0.1", port, workers, log_level, env, args.reload)


def uvicorn_args(config: RuntimeConfig) -> list[str]:
    """The uvicorn command line: `uvicorn task_tracker.app:app --host H --port P --workers N
    --log-level L`, plus `--reload` when enabled."""
    command = [
        "uvicorn",
        "task_tracker.app:app",
        "--host",
        config.host,
        "--port",
        str(config.port),
        "--workers",
        str(config.workers),
        "--log-level",
        config.log_level,
    ]
    return [*command, "--reload"] if config.reload else command
