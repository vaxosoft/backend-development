"""Runtime configuration for the service process: flags > environment > defaults."""

from collections.abc import Mapping, Sequence
from dataclasses import dataclass


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
    raise NotImplementedError


def uvicorn_args(config: RuntimeConfig) -> list[str]:
    """The uvicorn command line: `uvicorn task_tracker.app:app --host H --port P --workers N
    --log-level L`, plus `--reload` when enabled."""
    raise NotImplementedError
