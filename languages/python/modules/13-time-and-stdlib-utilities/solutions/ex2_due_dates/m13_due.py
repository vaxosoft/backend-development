"""Task Tracker time rules: RFC 3339 on the wire and date-only deadlines in a time zone."""

from datetime import UTC, date, datetime, time, timedelta
from zoneinfo import ZoneInfo


def _require_aware(value: datetime) -> None:
    if value.tzinfo is None or value.utcoffset() is None:
        raise ValueError("naive datetime: attach a time zone first")


def to_wire(value: datetime) -> str:
    """Format an aware datetime as UTC RFC 3339 with whole seconds: 2030-01-01T09:00:00Z.

    Raise ValueError for naive datetimes.
    """
    _require_aware(value)
    return value.astimezone(UTC).isoformat(timespec="seconds").replace("+00:00", "Z")


def from_wire(text: str) -> datetime:
    """Parse RFC 3339 (`Z` or a numeric offset, optional fractional seconds) into UTC.

    Raise ValueError when the text has no offset (naive) or is not a datetime.
    """
    parsed = datetime.fromisoformat(text)
    _require_aware(parsed)
    return parsed.astimezone(UTC)


def deadline(due: date, tz: ZoneInfo) -> datetime:
    """The UTC instant at which the calendar day `due` ends in `tz` (the next local midnight)."""
    next_midnight = datetime.combine(due + timedelta(days=1), time.min, tzinfo=tz)
    return next_midnight.astimezone(UTC)


def is_overdue(due: date | None, now: datetime, tz: ZoneInfo) -> bool:
    """True once `now` has reached the deadline. No due date is never overdue.

    Raise ValueError when `now` is naive.
    """
    _require_aware(now)
    return due is not None and now >= deadline(due, tz)


def time_left(due: date, now: datetime, tz: ZoneInfo) -> timedelta:
    """Time until the deadline; negative when overdue."""
    _require_aware(now)
    return deadline(due, tz) - now
