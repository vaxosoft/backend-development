"""Environment variables are strings: read them once, parse them, fail fast."""

import os
from unittest import mock


class ConfigError(Exception):
    pass


def env_bool(name: str, default: bool = False) -> bool:
    raw = os.environ.get(name)
    if raw is None:
        return default
    value = raw.strip().lower()
    if value in {"1", "true", "yes", "on"}:
        return True
    if value in {"0", "false", "no", "off", ""}:
        return False
    raise ConfigError(f"{name}: expected a boolean, got {raw!r}")


def env_int(name: str, default: int) -> int:
    raw = os.environ.get(name)
    if raw is None:
        return default
    try:
        return int(raw)
    except ValueError:
        raise ConfigError(f"{name}: expected an integer, got {raw!r}") from None


def env_required(name: str) -> str:
    try:
        return os.environ[name]
    except KeyError:
        raise ConfigError(f"{name} is required") from None


def main() -> None:
    # patch.dict gives the example a known environment; real code just reads os.environ
    environment = {"TT_DEBUG": "false", "TT_PORT": "9090", "TT_HOME": "/srv/tasks"}
    with mock.patch.dict(os.environ, environment, clear=True):
        # The classic trap: any non-empty string is truthy
        print("bool('false') is", bool(os.environ["TT_DEBUG"]))
        print("env_bool('TT_DEBUG') is", env_bool("TT_DEBUG"))

        print(
            "port:", env_int("TT_PORT", 8080), "| missing with default:", env_int("TT_WORKERS", 2)
        )
        print("os.environ.get for a missing name:", os.environ.get("TT_NOPE"))
        print("home:", env_required("TT_HOME"))

        try:
            env_required("TT_API_TOKEN")
        except ConfigError as exc:
            print("ConfigError:", exc)
        try:
            env_int("TT_HOME", 0)
        except ConfigError as exc:
            print("ConfigError:", exc)


if __name__ == "__main__":
    main()
