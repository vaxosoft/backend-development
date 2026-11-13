"""Exercise 2: per-request context with `contextvars`.

* `request_context(request_id=None, new_id=...)` is a context manager: inside the block
  `current_request_id()` returns the id; on exit (even after an exception) the PREVIOUS
  value is restored, so blocks can nest. It yields the id in use. When `request_id` is
  None, generate one by calling `new_id()`.
* `RequestIdFilter` is a `logging.Filter` that sets `record.request_id` to the current id
  (None outside a request) and never drops a record.
"""

import logging
import uuid
from collections.abc import Callable, Iterator
from contextlib import contextmanager
from contextvars import ContextVar

_request_id: ContextVar[str | None] = ContextVar("request_id", default=None)


def _default_id() -> str:
    return uuid.uuid4().hex


def current_request_id() -> str | None:
    raise NotImplementedError


@contextmanager
def request_context(
    request_id: str | None = None, new_id: Callable[[], str] = _default_id
) -> Iterator[str]:
    raise NotImplementedError
    yield ""  # makes this a generator function; keep it


class RequestIdFilter(logging.Filter):
    def filter(self, record: logging.LogRecord) -> bool:
        raise NotImplementedError
