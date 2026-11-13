from datetime import UTC, date, datetime, timedelta, timezone
from zoneinfo import ZoneInfo

import pytest
from m13_due import deadline, from_wire, is_overdue, time_left, to_wire

NEW_YORK = ZoneInfo("America/New_York")
AUCKLAND = ZoneInfo("Pacific/Auckland")


def test_to_wire_converts_to_utc_and_drops_microseconds() -> None:
    plus_two = datetime(2030, 1, 1, 11, 0, 5, 999_999, tzinfo=timezone(timedelta(hours=2)))
    assert to_wire(plus_two) == "2030-01-01T09:00:05Z"


def test_to_wire_rejects_naive() -> None:
    with pytest.raises(ValueError, match="naive"):
        to_wire(datetime(2030, 1, 1, 9, 0))


@pytest.mark.parametrize(
    "text",
    ["2030-01-01T09:00:00Z", "2030-01-01T11:00:00+02:00", "2030-01-01T04:00:00-05:00"],
)
def test_from_wire_normalises_to_utc(text: str) -> None:
    parsed = from_wire(text)
    assert parsed == datetime(2030, 1, 1, 9, 0, tzinfo=UTC)
    assert parsed.utcoffset() == timedelta(0)


def test_from_wire_accepts_fractions() -> None:
    assert from_wire("2030-01-01T09:00:00.5Z").microsecond == 500_000


@pytest.mark.parametrize("text", ["2030-01-01T09:00:00", "2030-01-01", "tomorrow", ""])
def test_from_wire_rejects_naive_or_garbage(text: str) -> None:
    with pytest.raises(ValueError, match=r"naive|Invalid isoformat"):
        from_wire(text)


def test_wire_round_trip() -> None:
    now = datetime(2030, 6, 1, 8, 30, tzinfo=NEW_YORK)
    assert from_wire(to_wire(now)) == now


def test_deadline_is_next_local_midnight_in_utc() -> None:
    assert deadline(date(2030, 1, 31), NEW_YORK) == datetime(2030, 2, 1, 5, 0, tzinfo=UTC)
    assert deadline(date(2030, 1, 31), AUCKLAND) == datetime(2030, 1, 31, 11, 0, tzinfo=UTC)
    assert deadline(date(2030, 1, 31), NEW_YORK).utcoffset() == timedelta(0)


def test_deadline_follows_daylight_saving_changes() -> None:
    # New York springs forward on 2030-03-10: the offset changes between these two deadlines.
    before = deadline(date(2030, 3, 9), NEW_YORK)
    on_the_day = deadline(date(2030, 3, 10), NEW_YORK)
    assert before == datetime(2030, 3, 10, 5, 0, tzinfo=UTC)
    assert on_the_day == datetime(2030, 3, 11, 4, 0, tzinfo=UTC)
    assert on_the_day - before == timedelta(hours=23)


def test_overdue_depends_on_the_zone() -> None:
    now = datetime(2030, 1, 31, 12, 0, tzinfo=UTC)
    assert is_overdue(date(2030, 1, 31), now, AUCKLAND)
    assert not is_overdue(date(2030, 1, 31), now, NEW_YORK)


def test_overdue_boundary_is_inclusive() -> None:
    due = date(2030, 1, 31)
    edge = datetime(2030, 2, 1, 5, 0, tzinfo=UTC)
    assert not is_overdue(due, edge - timedelta(seconds=1), NEW_YORK)
    assert is_overdue(due, edge, NEW_YORK)


def test_overdue_accepts_any_aware_offset() -> None:
    now = datetime(2030, 2, 1, 0, 0, tzinfo=NEW_YORK)
    assert is_overdue(date(2030, 1, 31), now, NEW_YORK)


def test_no_due_date_is_never_overdue() -> None:
    assert not is_overdue(None, datetime(2099, 1, 1, tzinfo=UTC), NEW_YORK)


def test_overdue_rejects_naive_now() -> None:
    with pytest.raises(ValueError, match="naive"):
        is_overdue(date(2030, 1, 31), datetime(2030, 2, 1), NEW_YORK)


def test_time_left_sign() -> None:
    due = date(2030, 1, 31)
    assert time_left(due, datetime(2030, 2, 1, 3, 0, tzinfo=UTC), NEW_YORK) == timedelta(hours=2)
    assert time_left(due, datetime(2030, 2, 1, 6, 0, tzinfo=UTC), NEW_YORK) == timedelta(hours=-1)
