from typing import Any

from .models import JobResult, JobType

MAX_SLEEP_MS = 5000


def validate_payload(job_type: JobType, payload: dict[str, Any]) -> None:
    """Raise InvalidPayload({"payload.<field>": message}) when the payload does not fit the type.

    reverse needs a string `text`; sleep needs an integer `ms` between 0 and 5000
    (a bool is not an integer here); fail accepts anything.
    """
    raise NotImplementedError("TODO: validate_payload")


async def run_job(job_type: JobType, payload: dict[str, Any]) -> JobResult:
    """Execute one attempt. The payload has already been validated.

    reverse -> the reversed text; sleep -> {"slept_ms": ms} after sleeping without blocking
    the event loop; fail -> always raises JobError.
    """
    raise NotImplementedError("TODO: run_job")
