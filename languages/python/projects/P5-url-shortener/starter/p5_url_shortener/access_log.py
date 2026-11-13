"""Structured access log: one JSON line per request."""

from fastapi import FastAPI

from .config import Config


def install_access_log(app: FastAPI, config: Config) -> None:
    """Add middleware that sends one JSON line per request to `config.log_sink`.

    Fields: request_id, method, path, status, duration_ms and (when known) user_id.
    Also answer with an `X-Request-ID` header: echo a sane incoming value, else generate one.
    Never log request bodies, query strings or the Authorization header.
    """
    raise NotImplementedError("TODO: add an HTTP middleware")
