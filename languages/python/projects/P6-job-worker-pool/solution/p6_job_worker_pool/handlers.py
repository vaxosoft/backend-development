import asyncio
from typing import Any

from .errors import InvalidPayload, JobError
from .models import JobResult, JobType

MAX_SLEEP_MS = 5000


def validate_payload(job_type: JobType, payload: dict[str, Any]) -> None:
    """Raise InvalidPayload({"payload.<field>": message}) when the payload does not fit the type.

    reverse needs a string `text`; sleep needs an integer `ms` between 0 and 5000
    (a bool is not an integer here); fail accepts anything.
    """
    match job_type:
        case "reverse":
            if not isinstance(payload.get("text"), str):
                raise InvalidPayload({"payload.text": "must be a string"})
        case "sleep":
            ms = payload.get("ms")
            if isinstance(ms, bool) or not isinstance(ms, int) or not 0 <= ms <= MAX_SLEEP_MS:
                raise InvalidPayload({"payload.ms": f"must be an integer from 0 to {MAX_SLEEP_MS}"})
        case "fail":
            pass


async def run_job(job_type: JobType, payload: dict[str, Any]) -> JobResult:
    """Execute one attempt. The payload has already been validated.

    reverse -> the reversed text; sleep -> {"slept_ms": ms} after sleeping without blocking
    the event loop; fail -> always raises JobError.
    """
    match job_type:
        case "reverse":
            return str(payload["text"])[::-1]
        case "sleep":
            ms = int(payload["ms"])
            await asyncio.sleep(ms / 1000)
            return {"slept_ms": ms}
        case "fail":
            raise JobError("job failed on purpose")
