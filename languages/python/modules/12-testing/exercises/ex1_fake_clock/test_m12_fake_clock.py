from datetime import UTC, datetime, timedelta

import pytest
from m12_fake_clock import DEFAULT_START, ExpiringCache, FakeClock


def test_clock_stands_still_until_told_otherwise() -> None:
    clock = FakeClock()

    assert clock() == DEFAULT_START
    assert clock() == DEFAULT_START


def test_clock_advance_and_set() -> None:
    clock = FakeClock()

    clock.advance(timedelta(minutes=90))
    assert clock() == datetime(2030, 1, 1, 10, 30, tzinfo=UTC)

    clock.set(datetime(2030, 2, 1, tzinfo=UTC))
    assert clock() == datetime(2030, 2, 1, tzinfo=UTC)


def test_clock_refuses_naive_datetimes_and_going_back() -> None:
    with pytest.raises(ValueError, match="naive"):
        FakeClock(datetime(2030, 1, 1))  # naive on purpose
    clock = FakeClock()
    with pytest.raises(ValueError, match="naive"):
        clock.set(datetime(2031, 1, 1))
    with pytest.raises(ValueError, match="backwards"):
        clock.advance(timedelta(seconds=-1))
    with pytest.raises(ValueError, match="backwards"):
        clock.set(DEFAULT_START - timedelta(days=1))


@pytest.fixture
def clock() -> FakeClock:
    return FakeClock()


@pytest.fixture
def cache(clock: FakeClock) -> ExpiringCache[str, int]:
    return ExpiringCache(timedelta(seconds=60), clock)


def test_value_is_available_until_the_ttl(cache: ExpiringCache[str, int], clock: FakeClock) -> None:
    cache.set("a", 1)

    clock.advance(timedelta(seconds=59))
    assert cache.get("a") == 1

    clock.advance(timedelta(seconds=1))  # exactly at the ttl: gone
    assert cache.get("a") is None


def test_unknown_key_is_none(cache: ExpiringCache[str, int]) -> None:
    assert cache.get("missing") is None


def test_setting_again_restarts_the_ttl(cache: ExpiringCache[str, int], clock: FakeClock) -> None:
    cache.set("a", 1)
    clock.advance(timedelta(seconds=40))
    cache.set("a", 2)
    clock.advance(timedelta(seconds=40))

    assert cache.get("a") == 2


def test_len_counts_only_live_entries(cache: ExpiringCache[str, int], clock: FakeClock) -> None:
    cache.set("old", 1)
    clock.advance(timedelta(seconds=30))
    cache.set("new", 2)
    assert len(cache) == 2

    clock.advance(timedelta(seconds=30))  # old expired, new has 30 s left

    assert len(cache) == 1


def test_purge_removes_expired_entries_and_reports_the_count(
    cache: ExpiringCache[str, int], clock: FakeClock
) -> None:
    cache.set("a", 1)
    cache.set("b", 2)
    clock.advance(timedelta(seconds=30))
    cache.set("c", 3)
    clock.advance(timedelta(seconds=30))

    assert cache.purge() == 2
    assert cache.purge() == 0
    assert cache.get("c") == 3
