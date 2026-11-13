"""Exceptions shared by the pool, the handlers and the HTTP layer."""


class JobError(Exception):
    """A job handler failed in an expected way; the pool may retry the job."""


class InvalidPayload(ValueError):
    """The payload does not match the job type. `fields` maps a field path to a message."""

    def __init__(self, fields: dict[str, str]) -> None:
        super().__init__(f"invalid payload: {fields}")
        self.fields = fields


class QueueFull(Exception):
    """The bounded queue has no free slot (HTTP 429)."""


class PoolClosed(Exception):
    """The pool is shutting down and no longer accepts jobs (HTTP 503)."""


class JobNotFound(Exception):
    """No job has this id (HTTP 404)."""


class JobFinished(Exception):
    """The job already reached a final status and cannot be cancelled (HTTP 409)."""
