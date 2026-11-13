"""Layering by hand: defaults < config file < environment < command line."""

import argparse
import tomllib
from collections import ChainMap
from collections.abc import Mapping, Sequence
from dataclasses import dataclass

DEFAULTS: dict[str, object] = {"host": "127.0.0.1", "port": 8080, "log_level": "info"}
LOG_LEVELS = ("debug", "info", "warn", "error")


class ConfigError(Exception):
    pass


@dataclass(slots=True, frozen=True)
class Config:
    host: str
    port: int
    log_level: str

    def __post_init__(self) -> None:
        if not 1 <= self.port <= 65535:
            raise ConfigError(f"port must be 1-65535, got {self.port}")
        if self.log_level not in LOG_LEVELS:
            raise ConfigError(f"log_level must be one of {LOG_LEVELS}, got {self.log_level!r}")


def from_env(env: Mapping[str, str]) -> dict[str, object]:
    mapping = {"host": "TT_HOST", "port": "TT_PORT", "log_level": "TT_LOG_LEVEL"}
    return {key: env[var] for key, var in mapping.items() if var in env}


def from_cli(argv: Sequence[str]) -> dict[str, object]:
    parser = argparse.ArgumentParser(exit_on_error=False)
    parser.add_argument("--host")
    parser.add_argument("--port")
    parser.add_argument("--log-level", dest="log_level")
    given = vars(parser.parse_args(argv))
    return {key: value for key, value in given.items() if value is not None}


def load(file_text: str, env: Mapping[str, str], argv: Sequence[str]) -> Config:
    # ChainMap looks a key up in each mapping in order: the first one that has it wins
    merged = ChainMap(from_cli(argv), from_env(env), tomllib.loads(file_text), DEFAULTS)
    try:
        return Config(str(merged["host"]), int(str(merged["port"])), str(merged["log_level"]))
    except ValueError as exc:
        raise ConfigError(f"port must be an integer: {exc}") from exc


def main() -> None:
    file_text = 'port = 9000\nlog_level = "warn"\n'
    env = {"TT_PORT": "9100", "TT_HOST": "0.0.0.0"}

    print("defaults only:     ", load("", {}, []))
    print("+ file:            ", load(file_text, {}, []))
    print("+ file + env:      ", load(file_text, env, []))
    print("+ file + env + cli:", load(file_text, env, ["--port", "9200", "--log-level", "debug"]))

    for label, args in (
        ("port=0", ["--port", "0"]),
        ("port=abc", ["--port", "abc"]),
        ("bad level", ["--log-level", "loud"]),
    ):
        try:
            load(file_text, env, args)
        except ConfigError as exc:
            print(f"{label}: ConfigError: {exc}")


if __name__ == "__main__":
    main()
