"""Correlation ids: a ContextVar gives every concurrent request its own id, with no plumbing."""

import asyncio
import logging
import sys

from m10_helpers import RequestIdFilter, request_id_var

log = logging.getLogger("tracker.worker")


async def handle(request_id: str, steps: int) -> None:
    token = request_id_var.set(request_id)  # visible only to this task and the code it calls
    try:
        for step in range(1, steps + 1):
            log.info("step %d of %d", step, steps)
            await asyncio.sleep(0)  # yield to the other requests
    finally:
        request_id_var.reset(token)


async def main() -> None:
    handler = logging.StreamHandler(sys.stdout)
    handler.addFilter(RequestIdFilter())
    handler.setFormatter(logging.Formatter("[%(request_id)s] %(message)s"))
    logging.basicConfig(level=logging.INFO, handlers=[handler])

    await asyncio.gather(handle("req-a", 2), handle("req-b", 3))
    log.info("outside any request")


if __name__ == "__main__":
    asyncio.run(main())
