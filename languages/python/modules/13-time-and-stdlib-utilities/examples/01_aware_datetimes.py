"""Aware vs naive datetimes, timedelta arithmetic and RFC 3339 on the wire."""

from datetime import UTC, datetime, timedelta, timezone

# Aware: knows its offset, so it names exactly one instant.
created_at = datetime(2030, 1, 1, 9, 0, tzinfo=UTC)
# Naive: wall-clock fields only. Which instant? Nobody knows.
naive = datetime(2030, 1, 1, 9, 0)

print("aware:", created_at.isoformat())
print("naive:", naive.isoformat())
print("naive.utcoffset():", naive.utcoffset())

try:
    created_at < naive  # noqa: B015
except TypeError as err:
    print("compare aware/naive ->", err)

# The same instant in two offsets compares equal.
plus_two = created_at.astimezone(timezone(timedelta(hours=2)))
print("same instant:", plus_two.isoformat(), plus_two == created_at)

# Arithmetic uses timedelta. Dates stay dates.
due = created_at + timedelta(days=30, hours=3)
print("due:", due.isoformat())
print("remaining:", due - created_at, "=", (due - created_at).total_seconds(), "seconds")

# RFC 3339 / ISO 8601 round trip (the Task Tracker wire format).
wire = created_at.isoformat().replace("+00:00", "Z")
print("wire:", wire)
parsed = datetime.fromisoformat("2030-01-01T11:00:00+02:00")
print("parsed:", parsed.astimezone(UTC).isoformat(), parsed == created_at)
print("fromisoformat accepts Z:", datetime.fromisoformat("2030-01-01T09:00:00Z") == created_at)

# datetime.utcnow() is deprecated: it returns a NAIVE value. Use now(UTC).
now = datetime.now(UTC)
print("now is aware:", now.tzinfo is not None)
