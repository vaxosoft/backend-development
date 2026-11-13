"""Parse and format short durations such as "1h30m" (used for timeouts and TTLs in config)."""

from datetime import timedelta


def parse_duration(text: str) -> timedelta:
    """Parse `[Nd][Nh][Nm][Ns]` (each unit at most once, in that order, at least one unit).

    Raise ValueError for anything else: empty text, spaces, unknown units, repeated or
    out-of-order units, signs and fractions.
    """
    raise NotImplementedError


def format_duration(value: timedelta) -> str:
    """Inverse of parse_duration: 5400 seconds -> "1h30m". Zero is "0s".

    Sub-second parts are dropped. Raise ValueError for negative durations.
    """
    raise NotImplementedError
