"""Exercise 1: a thread-safe "create once" cache."""

from collections.abc import Callable


class OnceCache[K, V]:
    """Maps keys to values that are expensive to build.

    Many threads may ask for the same key at the same time; the factory must still run
    exactly once per key and every caller must receive that same object.
    """

    def __init__(self) -> None:
        self._values: dict[K, V] = {}

    def get_or_create(self, key: K, factory: Callable[[], V]) -> V:
        """Return the cached value, calling `factory()` only if the key is missing.

        If the factory raises, nothing is cached and the exception propagates, so a later
        call tries again.
        """
        raise NotImplementedError

    def __len__(self) -> int:
        return len(self._values)
