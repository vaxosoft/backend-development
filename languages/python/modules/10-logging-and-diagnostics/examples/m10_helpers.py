"""Logging helpers shared by the examples of module 10."""

import json
import logging
from contextvars import ContextVar
from datetime import UTC, datetime
from typing import Any

request_id_var: ContextVar[str | None] = ContextVar("request_id", default=None)

# Attributes every LogRecord has; anything else was passed through `extra=`.
_STANDARD_ATTRS = frozenset(vars(logging.makeLogRecord({}))) | {
    "message",
    "asctime",
    "taskName",
    "color_message",  # added by uvicorn
}


class RequestIdFilter(logging.Filter):
    """Copy the current request id (a ContextVar) onto every record."""

    def filter(self, record: logging.LogRecord) -> bool:
        record.request_id = request_id_var.get()
        return True


class JsonFormatter(logging.Formatter):
    """One JSON object per line: level, logger, message, extra fields, exception."""

    def __init__(self, *, include_time: bool = True, compact_exceptions: bool = False) -> None:
        super().__init__()
        self.include_time = include_time
        self.compact_exceptions = compact_exceptions

    def format(self, record: logging.LogRecord) -> str:
        entry: dict[str, Any] = {}
        if self.include_time:
            entry["time"] = datetime.fromtimestamp(record.created, UTC).isoformat()
        entry["level"] = record.levelname.lower()
        entry["logger"] = record.name
        entry["message"] = record.getMessage()
        for key, value in vars(record).items():
            if key not in _STANDARD_ATTRS and value is not None:
                entry[key] = value
        if record.exc_info:
            exc_type, exc, _ = record.exc_info
            if self.compact_exceptions and exc_type is not None:
                entry["exception"] = f"{exc_type.__name__}: {exc}"
            else:
                entry["exception"] = self.formatException(record.exc_info)
        return json.dumps(entry, default=str)
