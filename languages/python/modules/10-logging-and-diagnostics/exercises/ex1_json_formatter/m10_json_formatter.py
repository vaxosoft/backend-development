"""Exercise 1: a JSON log formatter.

Implement `JsonFormatter.format` so each record becomes ONE line of JSON with:

* "level": the lower-case level name ("info", "error", ...)
* "logger": the logger name
* "message": the message with its %-arguments applied
* every field passed through `extra=` (but none of the standard LogRecord attributes)
* "exception": the formatted traceback, only when the record has `exc_info`

Values that JSON cannot represent must not crash the logger: fall back to `str()`.
"""

import logging


class JsonFormatter(logging.Formatter):
    def format(self, record: logging.LogRecord) -> str:
        raise NotImplementedError
