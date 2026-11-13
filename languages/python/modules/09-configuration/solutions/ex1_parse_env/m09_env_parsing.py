"""Exercise 1 (Easy): parse and validate raw environment strings."""

from collections.abc import Mapping

TRUE_WORDS = frozenset({"1", "true", "yes", "on"})
FALSE_WORDS = frozenset({"0", "false", "no", "off"})


class ConfigError(ValueError):
    pass


def require(env: Mapping[str, str], name: str) -> str:
    """Return env[name]. A missing or blank (whitespace-only) value raises
    ConfigError("<NAME> is required")."""
    value = env.get(name, "")
    if not value.strip():
        raise ConfigError(f"{name} is required")
    return value


def parse_bool(name: str, raw: str) -> bool:
    """Parse a boolean, ignoring case and surrounding whitespace.

    "1", "true", "yes", "on" -> True; "0", "false", "no", "off" -> False.
    Anything else (including "") raises ConfigError whose message contains `name`.
    """
    word = raw.strip().lower()
    if word in TRUE_WORDS:
        return True
    if word in FALSE_WORDS:
        return False
    raise ConfigError(f"{name}: expected a boolean, got {raw!r}")


def parse_int(
    name: str, raw: str, *, minimum: int | None = None, maximum: int | None = None
) -> int:
    """Parse an integer (surrounding whitespace is fine) and check the optional bounds.

    Not an integer, below `minimum` or above `maximum` raises ConfigError whose message
    contains `name`. The error must be a ConfigError, never a bare ValueError from int().
    """
    try:
        value = int(raw)
    except ValueError:
        raise ConfigError(f"{name}: expected an integer, got {raw!r}") from None
    if minimum is not None and value < minimum:
        raise ConfigError(f"{name}: must be >= {minimum}, got {value}")
    if maximum is not None and value > maximum:
        raise ConfigError(f"{name}: must be <= {maximum}, got {value}")
    return value


def parse_csv(raw: str) -> list[str]:
    """Split a comma-separated list: strip every item and drop empty ones.

    "a, b,,c " -> ["a", "b", "c"]; "" -> [].
    """
    return [item.strip() for item in raw.split(",") if item.strip()]
