from itertools import count, islice

import pytest
from m01_unit_of_work import KeyValueStore, paginate, retry


def test_transaction_commits_on_success() -> None:
    store = KeyValueStore()
    with store.transaction() as tx:
        tx.set("a", 1)
        tx.set("b", 2)
    assert store.keys() == ["a", "b"]


def test_transaction_yields_the_store_itself() -> None:
    store = KeyValueStore()
    with store.transaction() as tx:
        assert tx is store


def test_transaction_rolls_back_and_reraises() -> None:
    store = KeyValueStore()
    store.set("keep", {"tags": ["x"]})
    store.set("gone", 1)

    def failing_block() -> None:
        with store.transaction() as tx:
            tx.set("new", 1)
            tx.delete("gone")
            tx.get("keep")["tags"].append("y")
            raise RuntimeError("boom")

    with pytest.raises(RuntimeError, match="boom"):
        failing_block()

    assert store.keys() == ["gone", "keep"]
    assert store.get("keep") == {"tags": ["x"]}


def test_store_is_usable_after_rollback() -> None:
    store = KeyValueStore()

    def failing_block() -> None:
        with store.transaction() as tx:
            tx.set("a", 1)
            raise ValueError("x")

    with pytest.raises(ValueError, match="x"):
        failing_block()
    with store.transaction() as tx:
        tx.set("b", 2)
    assert store.keys() == ["b"]


def test_paginate_chunks() -> None:
    assert list(paginate(range(7), 3)) == [[0, 1, 2], [3, 4, 5], [6]]
    assert list(paginate([], 3)) == []


def test_paginate_is_lazy() -> None:
    pages = paginate(count(), 2)
    assert list(islice(pages, 3)) == [[0, 1], [2, 3], [4, 5]]


def test_paginate_rejects_bad_size() -> None:
    with pytest.raises(ValueError, match="size"):
        list(paginate([1, 2], 0))


def test_retry_succeeds_after_failures() -> None:
    calls = 0

    @retry(attempts=3, on=ConnectionError)
    def flaky(x: int) -> int:
        """Doubles x."""
        nonlocal calls
        calls += 1
        if calls < 3:
            raise ConnectionError("down")
        return x * 2

    assert flaky(21) == 42
    assert calls == 3
    assert flaky.__name__ == "flaky"
    assert flaky.__doc__ == "Doubles x."


def test_retry_gives_up_with_last_error() -> None:
    calls = 0

    @retry(attempts=2, on=ConnectionError)
    def always_fails() -> None:
        nonlocal calls
        calls += 1
        raise ConnectionError(f"try {calls}")

    with pytest.raises(ConnectionError, match="try 2"):
        always_fails()
    assert calls == 2


def test_retry_does_not_catch_other_errors() -> None:
    calls = 0

    @retry(attempts=5, on=ConnectionError)
    def wrong_kind() -> None:
        nonlocal calls
        calls += 1
        raise KeyError("k")

    with pytest.raises(KeyError):
        wrong_kind()
    assert calls == 1
