import logging
from collections.abc import Mapping
from dataclasses import dataclass

LOG_LEVELS = {
    "debug": logging.DEBUG,
    "info": logging.INFO,
    "warn": logging.WARNING,
    "error": logging.ERROR,
}
ENVIRONMENTS = ("dev", "test", "prod")


class ConfigError(Exception):
    """The environment holds an invalid setting. The message names every offending variable."""


@dataclass(slots=True, frozen=True)
class Config:
    host: str = "127.0.0.1"
    port: int = 8080
    log_level: str = "info"
    env: str = "dev"

    @property
    def logging_level(self) -> int:
        return LOG_LEVELS[self.log_level]


def load_config(env: Mapping[str, str]) -> Config:
    """Build a Config from environment variables (pass `os.environ`, or a dict in tests).

    HOST (default 127.0.0.1), PORT (1-65535, default 8080), LOG_LEVEL (debug|info|warn|error,
    case-insensitive, default info) and ENV (dev|test|prod, default dev).
    Collect *every* problem and raise one ConfigError listing them, e.g.
    "invalid configuration: PORT must be an integer from 1 to 65535, got 'abc'; ...".
    """
    defaults = Config()
    problems: list[str] = []

    raw_port = env.get("PORT", str(defaults.port))
    port = defaults.port
    try:
        port = int(raw_port)
        if not 1 <= port <= 65535:
            raise ValueError(raw_port)
    except ValueError:
        problems.append(f"PORT must be an integer from 1 to 65535, got {raw_port!r}")

    log_level = env.get("LOG_LEVEL", defaults.log_level).lower()
    if log_level not in LOG_LEVELS:
        problems.append(f"LOG_LEVEL must be one of {', '.join(LOG_LEVELS)}, got {log_level!r}")

    app_env = env.get("ENV", defaults.env)
    if app_env not in ENVIRONMENTS:
        problems.append(f"ENV must be one of {', '.join(ENVIRONMENTS)}, got {app_env!r}")

    host = env.get("HOST", defaults.host)
    if not host.strip():
        problems.append(f"HOST must not be empty, got {host!r}")

    if problems:
        raise ConfigError("invalid configuration: " + "; ".join(problems))
    return Config(host=host, port=port, log_level=log_level, env=app_env)
