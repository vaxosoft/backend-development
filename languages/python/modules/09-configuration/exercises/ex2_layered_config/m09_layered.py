"""Exercise 2 (Medium): layered configuration with one validation pass."""

import argparse
from collections.abc import Mapping, Sequence
from dataclasses import dataclass

LOG_LEVELS = ("debug", "info", "warn", "error")
BOOL_WORDS = {"1": True, "true": True, "yes": True, "on": True, "0": False, "false": False}
BOOL_WORDS |= {"no": False, "off": False}
DEFAULTS: dict[str, object] = {
    "host": "127.0.0.1",
    "port": 8080,
    "log_level": "info",
    "debug": False,
}
ENV_VARS = {
    "host": "APP_HOST",
    "port": "APP_PORT",
    "log_level": "APP_LOG_LEVEL",
    "debug": "APP_DEBUG",
}


class ConfigError(ValueError):
    def __init__(self, errors: Sequence[str]) -> None:
        super().__init__("; ".join(errors))
        self.errors = list(errors)


@dataclass(slots=True, frozen=True)
class Config:
    host: str
    port: int
    log_level: str
    debug: bool


def build_parser() -> argparse.ArgumentParser:
    """Every flag defaults to None, which means "not given on the command line"."""
    parser = argparse.ArgumentParser(prog="tracker", exit_on_error=False)
    parser.add_argument("--host")
    parser.add_argument("--port")
    parser.add_argument("--log-level", dest="log_level")
    parser.add_argument("--debug", action=argparse.BooleanOptionalAction)
    return parser


def load_config(
    *, file_values: Mapping[str, object], env: Mapping[str, str], argv: Sequence[str]
) -> Config:
    """Merge four layers, lowest to highest priority, and validate the result.

    1. DEFAULTS
    2. `file_values`: a parsed TOML file with native types (port=9000, debug=true)
    3. `env`: APP_HOST, APP_PORT, APP_LOG_LEVEL, APP_DEBUG (see ENV_VARS), all strings
    4. `argv`: parsed with build_parser(); only flags that were given count

    Validation applies to the winning value of each key only (a bad value that a higher
    layer overrides is ignored):

    - host: non-empty string
    - port: integer (or digit string) from 1 to 65535
    - log_level: one of LOG_LEVELS
    - debug: a bool, or a string found in BOOL_WORDS (case-insensitive)

    Report every problem together: raise ConfigError(errors) with one message per problem,
    each containing the key name. Unknown keys in `file_values` are errors too
    ("unknown key 'colour' in config file"). A command-line parsing failure
    (argparse.ArgumentError) becomes ConfigError with that single message; never SystemExit.
    """
    raise NotImplementedError("TODO: implement load_config")
