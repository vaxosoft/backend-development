"""zoneinfo: store UTC, convert at the edges, and survive daylight saving time."""

from datetime import UTC, date, datetime, time, timedelta
from zoneinfo import ZoneInfo

berlin = ZoneInfo("Europe/Berlin")
new_york = ZoneInfo("America/New_York")

instant = datetime(2030, 7, 1, 12, 0, tzinfo=UTC)
print("UTC:     ", instant.isoformat())
print("Berlin:  ", instant.astimezone(berlin).isoformat())
print("New York:", instant.astimezone(new_york).isoformat())

# Wall-clock arithmetic keeps the local time; absolute arithmetic keeps the instant.
before = datetime(2030, 3, 30, 12, 0, tzinfo=berlin)  # the day before the spring-forward
wall = before + timedelta(days=1)
absolute = (before.astimezone(UTC) + timedelta(days=1)).astimezone(berlin)
print("wall clock +1 day:", wall.isoformat())
print("absolute   +1 day:", absolute.isoformat())

# A local time that does not exist (clock jumps 02:00 -> 03:00) is not rejected:
gap = datetime(2030, 3, 31, 2, 30, tzinfo=berlin)
print("nonexistent 02:30 ->", gap.astimezone(UTC).isoformat())

# A local time that happens twice (03:00 -> 02:00) is disambiguated with `fold`.
first = datetime(2030, 10, 27, 2, 30, tzinfo=berlin, fold=0)
second = datetime(2030, 10, 27, 2, 30, tzinfo=berlin, fold=1)
print("ambiguous 02:30 fold=0:", first.isoformat())
print("ambiguous 02:30 fold=1:", second.isoformat())

# A date-only due date has no instant until you choose a zone.
due = date(2030, 1, 31)
end_of_day = datetime.combine(due + timedelta(days=1), time.min, tzinfo=new_york)
print("due", due, "ends at", end_of_day.astimezone(UTC).isoformat())
