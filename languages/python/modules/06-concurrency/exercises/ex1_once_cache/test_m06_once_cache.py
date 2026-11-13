import threading
import time
from collections.abc import Callable

import pytest
from m06_once_cache import OnceCache

THREADS = 16


def run_concurrently(target: Callable[[int], None], count: int = THREADS) -> None:
    """Start `count` threads together (barrier) and wait for all of them."""
    start = threading.Barrier(count)

    def body(i: int) -> None:
        start.wait()
        target(i)

    threads = [threading.Thread(target=body, args=(i,)) for i in range(count)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()


def test_factory_runs_once_even_when_many_threads_ask_at_once() -> None:
    cache: OnceCache[str, object] = OnceCache()
    calls: list[int] = []
    seen: list[object] = []

    def factory() -> object:
        calls.append(1)
        time.sleep(0.01)  # widen the window in which a second caller could slip in
        return object()

    def ask(_: int) -> None:
        seen.append(cache.get_or_create("config", factory))

    run_concurrently(ask)
    assert len(calls) == 1
    assert len(seen) == THREADS
    assert all(value is seen[0] for value in seen)


def test_each_key_gets_its_own_value() -> None:
    cache: OnceCache[int, int] = OnceCache()
    calls: dict[int, int] = {}
    seen: list[tuple[int, int]] = []

    def ask(i: int) -> None:
        key = i % 4

        def factory() -> int:
            calls[key] = calls.get(key, 0) + 1
            time.sleep(0.005)
            return key * 10

        seen.append((key, cache.get_or_create(key, factory)))

    run_concurrently(ask)
    assert calls == {0: 1, 1: 1, 2: 1, 3: 1}
    assert sorted(seen) == sorted((i % 4, (i % 4) * 10) for i in range(THREADS))
    assert len(cache) == 4


def test_cached_value_skips_the_factory() -> None:
    cache: OnceCache[str, int] = OnceCache()
    assert cache.get_or_create("a", lambda: 1) == 1

    def must_not_run() -> int:
        raise AssertionError("factory called for a cached key")

    assert cache.get_or_create("a", must_not_run) == 1


def test_failed_factory_is_not_cached_and_can_be_retried() -> None:
    cache: OnceCache[str, int] = OnceCache()

    def broken() -> int:
        raise RuntimeError("boom")

    with pytest.raises(RuntimeError, match="boom"):
        cache.get_or_create("k", broken)
    assert len(cache) == 0
    assert cache.get_or_create("k", lambda: 7) == 7
