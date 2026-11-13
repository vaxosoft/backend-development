import asyncio
import logging
from itertools import count

import pytest
from m10_request_context import RequestIdFilter, current_request_id, request_context


def test_id_is_visible_inside_and_gone_outside() -> None:
    assert current_request_id() is None
    with request_context("req-1") as rid:
        assert rid == "req-1"
        assert current_request_id() == "req-1"
    assert current_request_id() is None


def test_generates_an_id_with_the_injected_factory() -> None:
    ids = (f"gen-{n}" for n in count(1))

    with request_context(new_id=lambda: next(ids)) as first:
        assert current_request_id() == first == "gen-1"
    with request_context(new_id=lambda: next(ids)) as second:
        assert second == "gen-2"


def test_nested_contexts_restore_the_outer_id() -> None:
    with request_context("outer"):
        with request_context("inner"):
            assert current_request_id() == "inner"
        assert current_request_id() == "outer"


def test_id_is_reset_even_when_the_block_raises() -> None:
    with pytest.raises(RuntimeError), request_context("req-err"):
        raise RuntimeError("boom")

    assert current_request_id() is None


def test_filter_stamps_records_and_keeps_them() -> None:
    record = logging.makeLogRecord({"msg": "hello"})
    log_filter = RequestIdFilter()

    with request_context("req-9"):
        assert log_filter.filter(record) is True
    assert record.request_id == "req-9"  # type: ignore[attr-defined]

    outside = logging.makeLogRecord({"msg": "idle"})
    assert log_filter.filter(outside) is True
    assert outside.request_id is None  # type: ignore[attr-defined]


def test_concurrent_tasks_do_not_see_each_others_ids() -> None:
    seen: dict[str, list[str | None]] = {"a": [], "b": []}

    async def handle(name: str) -> None:
        with request_context(f"req-{name}"):
            for _ in range(3):
                seen[name].append(current_request_id())
                await asyncio.sleep(0)

    async def main() -> None:
        await asyncio.gather(handle("a"), handle("b"))

    asyncio.run(main())

    assert seen == {"a": ["req-a"] * 3, "b": ["req-b"] * 3}
    assert current_request_id() is None
