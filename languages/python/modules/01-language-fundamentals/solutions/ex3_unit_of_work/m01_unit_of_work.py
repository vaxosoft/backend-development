"""Exercise 3: context managers, generators and decorators."""

import copy
from collections.abc import Callable, Iterable, Iterator
from contextlib import contextmanager
from functools import wraps
from itertools import batched
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
        snapshot = copy.deepcopy(self._data)
        try:
            yield self
        except BaseException:
            self._data = snapshot
            raise


def paginate[T](items: Iterable[T], size: int) -> Iterator[list[T]]:
    """Lazily yield lists of at most `size` items. Must work on infinite iterables.

    Raise ValueError (when first iterated or called, either is fine) if size < 1.
    """
    if size < 1:
        raise ValueError("size must be >= 1")
    return (list(chunk) for chunk in batched(items, size, strict=False))


def retry[**P, R](attempts: int, on: type[Exception]) -> Callable[[Callable[P, R]], Callable[P, R]]:
    """Decorator factory: call the function up to `attempts` times while it raises `on`.

    Re-raise the last error when all attempts fail. Other exceptions propagate immediately.
    The wrapper must keep the wrapped function's __name__ and __doc__.
    """

    def decorator(func: Callable[P, R]) -> Callable[P, R]:
        @wraps(func)
        def wrapper(*args: P.args, **kwargs: P.kwargs) -> R:
            for attempt in range(1, attempts + 1):
                try:
                    return func(*args, **kwargs)
                except on:
                    if attempt == attempts:
                        raise
            raise ValueError("attempts must be >= 1")

        return wrapper

    return decorator
