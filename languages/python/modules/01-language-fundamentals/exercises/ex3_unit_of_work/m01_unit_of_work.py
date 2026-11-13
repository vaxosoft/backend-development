"""Exercise 3: context managers, generators and decorators."""

from collections.abc import Callable, Iterable, Iterator
from contextlib import contextmanager
from typing import Any


class KeyValueStore:
    def __init__(self) -> None:
        self._data: dict[str, Any] = {}

    def set(self, key: str, value: Any) -> None:
        self._data[key] = value

    def get(self, key: str) -> Any:
        return self._data.get(key)

    def delete(self, key: str) -> None:
        self._data.pop(key, None)

    def keys(self) -> list[str]:
        return sorted(self._data)

    @contextmanager
    def transaction(self) -> Iterator[KeyValueStore]:
        """Yield the store. If the block raises, restore every key to its state before the block
        (including deleted keys and mutated nested values) and re-raise the same exception.
        """
        raise NotImplementedError
        yield self


def paginate[T](items: Iterable[T], size: int) -> Iterator[list[T]]:
    """Lazily yield lists of at most `size` items. Must work on infinite iterables.

    Raise ValueError (when first iterated or called, either is fine) if size < 1.
    """
    raise NotImplementedError


def retry[**P, R](attempts: int, on: type[Exception]) -> Callable[[Callable[P, R]], Callable[P, R]]:
    """Decorator factory: call the function up to `attempts` times while it raises `on`.

    Re-raise the last error when all attempts fail. Other exceptions propagate immediately.
    The wrapper must keep the wrapped function's __name__ and __doc__.
    """
    raise NotImplementedError
