from datetime import timedelta

import pytest
from m13_duration import format_duration, parse_duration


@pytest.mark.parametrize(
    ("text", "expected"),
    [
        ("90s", timedelta(seconds=90)),
        ("1h30m", timedelta(hours=1, minutes=30)),
        ("2d", timedelta(days=2)),
        ("1d2h3m4s", timedelta(days=1, hours=2, minutes=3, seconds=4)),
        ("0s", timedelta(0)),
        ("007m", timedelta(minutes=7)),
    ],
)
def test_parse_valid(text: str, expected: timedelta) -> None:
    assert parse_duration(text) == expected


@pytest.mark.parametrize(
    "text",
    ["", "h", "5", "1x", "1h 30m", "30m1h", "1h1h", "-5m", "+5m", "1.5h", "1H", "5s ", "\n5s"],
)
def test_parse_rejects_invalid(text: str) -> None:
    with pytest.raises(ValueError, match="invalid duration"):
        parse_duration(text)


@pytest.mark.parametrize(
    ("value", "expected"),
    [
        (timedelta(0), "0s"),
        (timedelta(seconds=5400), "1h30m"),
        (timedelta(seconds=59), "59s"),
        (timedelta(days=1, seconds=1), "1d1s"),
        (timedelta(hours=48), "2d"),
        (timedelta(seconds=1, milliseconds=999), "1s"),
    ],
)
def test_format(value: timedelta, expected: str) -> None:
    assert format_duration(value) == expected


def test_format_rejects_negative() -> None:
    with pytest.raises(ValueError, match="negative"):
        format_duration(timedelta(seconds=-1))


@pytest.mark.parametrize("seconds", [1, 59, 60, 3599, 3600, 86399, 86400, 90061])
def test_round_trip(seconds: int) -> None:
    value = timedelta(seconds=seconds)
    assert parse_duration(format_duration(value)) == value
