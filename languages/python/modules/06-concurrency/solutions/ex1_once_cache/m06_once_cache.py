import threading
from collections.abc import Callable


class OnceCache[K, V]:
    def __init__(self) -> None:
        self._values: dict[K, V] = {}
        self._lock = threading.Lock()

    def get_or_create(self, key: K, factory: Callable[[], V]) -> V:
        # The check and the creation must be one atomic step, so the factory runs under the lock.
        with self._lock:
            if key not in self._values:
                self._values[key] = factory()
            return self._values[key]

    def __len__(self) -> int:
        return len(self._values)
