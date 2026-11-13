"""Task Tracker time rules: RFC 3339 on the wire and date-only deadlines in a time zone."""

from datetime import date, datetime, timedelta
from zoneinfo import ZoneInfo


def to_wire(value: datetime) -> str:
    """Format an aware datetime as UTC RFC 3339 with whole seconds: 2030-01-01T09:00:00Z.

    Raise ValueError for naive datetimes.
    """
    raise NotImplementedError


def from_wire(text: str) -> datetime:
    """Parse RFC 3339 (`Z` or a numeric offset, optional fractional seconds) into UTC.

    Raise ValueError when the text has no offset (naive) or is not a datetime.
    """
    raise NotImplementedError


def deadline(due: date, tz: ZoneInfo) -> datetime:
    """The UTC instant at which the calendar day `due` ends in `tz` (the next local midnight)."""
    raise NotImplementedError


def is_overdue(due: date | None, now: datetime, tz: ZoneInfo) -> bool:
    """True once `now` has reached the deadline. No due date is never overdue.

    Raise ValueError when `now` is naive.
    """
    raise NotImplementedError


def time_left(due: date, now: datetime, tz: ZoneInfo) -> timedelta:
    """Time until the deadline; negative when overdue."""
    raise NotImplementedError
