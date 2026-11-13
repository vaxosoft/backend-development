"""Exercise 1: a JSON log formatter.

Implement `JsonFormatter.format` so each record becomes ONE line of JSON with:

* "level": the lower-case level name ("info", "error", ...)
* "logger": the logger name
* "message": the message with its %-arguments applied
* every field passed through `extra=` (but none of the standard LogRecord attributes)
* "exception": the formatted traceback, only when the record has `exc_info`

Values that JSON cannot represent must not crash the logger: fall back to `str()`.
"""

import json
import logging
from typing import Any

# Everything a plain LogRecord already carries; what is left was passed through `extra=`.
_STANDARD_ATTRS = frozenset(vars(logging.makeLogRecord({}))) | {"message", "asctime", "taskName"}


class JsonFormatter(logging.Formatter):
    def format(self, record: logging.LogRecord) -> str:
        entry: dict[str, Any] = {
            "level": record.levelname.lower(),
            "logger": record.name,
            "message": record.getMessage(),
        }
        for key, value in vars(record).items():
            if key not in _STANDARD_ATTRS:
                entry[key] = value
        if record.exc_info:
            entry["exception"] = self.formatException(record.exc_info)
        return json.dumps(entry, default=str)
