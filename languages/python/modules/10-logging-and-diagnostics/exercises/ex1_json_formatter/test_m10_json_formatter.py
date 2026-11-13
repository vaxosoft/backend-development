import io
import json
import logging
from collections.abc import Iterator
from datetime import date
from typing import Any

import pytest
from m10_json_formatter import JsonFormatter


class Capture:
    def __init__(self) -> None:
        self.stream = io.StringIO()
        handler = logging.StreamHandler(self.stream)
        handler.setFormatter(JsonFormatter())
        self.logger = logging.getLogger("tracker.test.json")
        self.logger.handlers = [handler]
        self.logger.propagate = False
        self.logger.setLevel(logging.DEBUG)

    def lines(self) -> list[dict[str, Any]]:
        return [json.loads(line) for line in self.stream.getvalue().splitlines()]


@pytest.fixture
def capture() -> Iterator[Capture]:
    cap = Capture()
    yield cap
    cap.logger.handlers = []


def test_core_fields(capture: Capture) -> None:
    capture.logger.warning("disk at %d%%", 91)

    assert capture.lines() == [
        {"level": "warning", "logger": "tracker.test.json", "message": "disk at 91%"}
    ]


def test_extra_fields_are_included_and_standard_attributes_are_not(capture: Capture) -> None:
    capture.logger.info("task created", extra={"task_id": 7, "tags": ["docs"]})

    [entry] = capture.lines()
    assert entry["task_id"] == 7
    assert entry["tags"] == ["docs"]
    assert set(entry) == {"level", "logger", "message", "task_id", "tags"}


def test_exception_is_a_field_of_the_same_line(capture: Capture) -> None:
    try:
        int("abc")
    except ValueError:
        capture.logger.exception("bad number")

    assert capture.stream.getvalue().count("\n") == 1  # newlines inside are JSON-escaped
    [entry] = capture.lines()
    assert entry["level"] == "error"
    assert entry["exception"].startswith("Traceback")
    assert "ValueError: invalid literal" in entry["exception"]


def test_no_exception_key_without_exc_info(capture: Capture) -> None:
    capture.logger.info("fine")

    assert "exception" not in capture.lines()[0]


def test_values_json_cannot_encode_fall_back_to_str(capture: Capture) -> None:
    capture.logger.info("due", extra={"due": date(2030, 1, 31), "ids": {1}})

    [entry] = capture.lines()
    assert entry["due"] == "2030-01-31"
    assert entry["ids"] == "{1}"
