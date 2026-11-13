"""Parse and format short durations such as "1h30m" (used for timeouts and TTLs in config)."""

import re
from datetime import timedelta

_DURATION = re.compile(r"(?:(?P<d>\d+)d)?(?:(?P<h>\d+)h)?(?:(?P<m>\d+)m)?(?:(?P<s>\d+)s)?")


def parse_duration(text: str) -> timedelta:
    """Parse `[Nd][Nh][Nm][Ns]` (each unit at most once, in that order, at least one unit).

    Raise ValueError for anything else: empty text, spaces, unknown units, repeated or
    out-of-order units, signs and fractions.
    """
    match = _DURATION.fullmatch(text)
    if not text or match is None:
        raise ValueError(f"invalid duration: {text!r}")
    parts = {unit: int(number or 0) for unit, number in match.groupdict().items()}
    return timedelta(days=parts["d"], hours=parts["h"], minutes=parts["m"], seconds=parts["s"])


def format_duration(value: timedelta) -> str:
    """Inverse of parse_duration: 5400 seconds -> "1h30m". Zero is "0s".

    Sub-second parts are dropped. Raise ValueError for negative durations.
    """
    if value < timedelta(0):
        raise ValueError("negative duration")
    seconds = int(value.total_seconds())
    days, seconds = divmod(seconds, 86400)
    hours, seconds = divmod(seconds, 3600)
    minutes, seconds = divmod(seconds, 60)
    text = "".join(
        f"{amount}{unit}"
        for amount, unit in ((days, "d"), (hours, "h"), (minutes, "m"), (seconds, "s"))
        if amount
    )
    return text or "0s"
