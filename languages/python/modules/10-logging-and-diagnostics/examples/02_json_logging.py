"""Structured logging: one JSON object per line, with contextual fields via `extra=`."""

import logging
import sys

from m10_helpers import JsonFormatter

log = logging.getLogger("tracker.tasks")


def main() -> None:
    handler = logging.StreamHandler(sys.stdout)
    # No timestamp and one-line exceptions keep this demo's output stable.
    handler.setFormatter(JsonFormatter(include_time=False, compact_exceptions=True))
    logging.basicConfig(level=logging.INFO, handlers=[handler])

    log.info("task created", extra={"task_id": 7, "status": "todo", "tags": ["docs", "p1"]})
    log.warning("rejected %s", "empty title", extra={"field": "title"})

    # A LoggerAdapter attaches the same fields to every call of one unit of work.
    # merge_extra=True (3.13+) combines them with the per-call extra=.
    job = logging.LoggerAdapter(log, {"job": "nightly-export"}, merge_extra=True)
    job.info("started")
    job.info("finished", extra={"rows": 120})

    try:
        int("abc")
    except ValueError:
        log.exception("bad number", extra={"input": "abc"})


if __name__ == "__main__":
    main()
