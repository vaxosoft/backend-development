"""Exercise 1: avoid Python's classic reference and closure traps."""

import copy
from collections.abc import Callable
from typing import Any


def add_tag(tag: str, tags: list[str] | None = None) -> list[str]:
    """Return a NEW list with `tag` appended. Never mutate `tags` or share state between calls."""
    return [*(tags or []), tag]


def make_multipliers(count: int) -> list[Callable[[int], int]]:
    """Return `count` functions; function `i` multiplies its argument by `i`."""

    def make(factor: int) -> Callable[[int], int]:
        return lambda x: x * factor

    return [make(i) for i in range(count)]


def remove_all(items: list[int], value: int) -> list[int]:
    """Return a new list without any occurrence of `value`; leave `items` untouched."""
    return [item for item in items if item != value]


def merge_settings(base: dict[str, Any], override: dict[str, Any]) -> dict[str, Any]:
    """Return a new dict: `override` wins; nested dicts are merged recursively.

    Neither input may be modified, and the result must not share nested containers with them.
    """
    result = copy.deepcopy(base)
    for key, value in override.items():
        if isinstance(value, dict) and isinstance(result.get(key), dict):
            result[key] = merge_settings(result[key], value)
        else:
            result[key] = copy.deepcopy(value)
    return result
