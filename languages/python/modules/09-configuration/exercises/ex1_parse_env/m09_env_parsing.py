"""Exercise 1 (Easy): parse and validate raw environment strings."""

from collections.abc import Mapping


class ConfigError(ValueError):
    pass


def require(env: Mapping[str, str], name: str) -> str:
    """Return env[name]. A missing or blank (whitespace-only) value raises
    ConfigError("<NAME> is required")."""
    raise NotImplementedError("TODO: implement require")


def parse_bool(name: str, raw: str) -> bool:
    """Parse a boolean, ignoring case and surrounding whitespace.

    "1", "true", "yes", "on" -> True; "0", "false", "no", "off" -> False.
    Anything else (including "") raises ConfigError whose message contains `name`.
    """
    raise NotImplementedError("TODO: implement parse_bool")


def parse_int(
    name: str, raw: str, *, minimum: int | None = None, maximum: int | None = None
) -> int:
    """Parse an integer (surrounding whitespace is fine) and check the optional bounds.

    Not an integer, below `minimum` or above `maximum` raises ConfigError whose message
    contains `name`. The error must be a ConfigError, never a bare ValueError from int().
    """
    raise NotImplementedError("TODO: implement parse_int")


def parse_csv(raw: str) -> list[str]:
    """Split a comma-separated list: strip every item and drop empty ones.

    "a, b,,c " -> ["a", "b", "c"]; "" -> [].
    """
    raise NotImplementedError("TODO: implement parse_csv")
