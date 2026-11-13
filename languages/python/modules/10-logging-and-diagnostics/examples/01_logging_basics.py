"""Loggers, levels, handlers, formatters: the stdlib `logging` model in one file."""

import logging
import sys
from types import TracebackType

type ExcInfo = (
    tuple[type[BaseException], BaseException, TracebackType | None] | tuple[None, None, None]
)

# A library module only creates a logger; it never configures handlers.
library_log = logging.getLogger("tracker.store")
library_log.addHandler(logging.NullHandler())

log = logging.getLogger("tracker.api")


class OneLineTracebackFormatter(logging.Formatter):
    """Keeps this demo's output stable; real services keep the full traceback."""

    def formatException(self, ei: ExcInfo) -> str:
        exc_type, exc, _ = ei
        return f"{exc_type.__name__}: {exc}" if exc_type else str(exc)


def configure() -> None:
    """The application (never a library) configures logging once, at startup."""
    handler = logging.StreamHandler(sys.stdout)
    handler.setFormatter(OneLineTracebackFormatter("%(levelname)-7s %(name)s: %(message)s"))
    root = logging.getLogger()
    root.addHandler(handler)
    root.setLevel(logging.INFO)
    logging.getLogger("tracker.store").setLevel(logging.DEBUG)


def main() -> None:
    configure()

    log.debug("dropped: the root level is INFO")
    log.info("server starting on %s:%d", "127.0.0.1", 8080)  # lazy %-formatting
    log.warning("slow query took %.1f ms", 412.345)

    library_log.debug("kept: this logger has its own DEBUG level")

    settings: dict[str, str] = {}
    try:
        settings["port"]
    except KeyError:
        log.exception("lookup failed")  # ERROR + traceback, only inside `except`

    log.info("DEBUG enabled for tracker.api? %s", log.isEnabledFor(logging.DEBUG))


if __name__ == "__main__":
    main()
