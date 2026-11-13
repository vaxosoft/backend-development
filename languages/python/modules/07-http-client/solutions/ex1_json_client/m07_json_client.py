"""Exercise 1 (Easy): a small JSON helper that turns the error envelope into an exception."""

import httpx


class ApiError(Exception):
    def __init__(self, status: int, code: str, message: str) -> None:
        super().__init__(f"{status} {code}: {message}")
        self.status = status
        self.code = code
        self.message = message


def request_json(
    client: httpx.Client,
    method: str,
    path: str,
    *,
    token: str | None = None,
    params: dict[str, str | int] | None = None,
    json: object = None,
) -> object:
    """Send a request and return the decoded JSON body.

    - `token` adds `Authorization: Bearer <token>`.
    - 2xx: return the parsed JSON, or None when the body is empty (for example 204).
    - Anything else: raise ApiError(status, code, message) built from the shared error
      envelope {"error": {"code": ..., "message": ...}}. When the body is not that
      envelope, use code "http_error" and the response's reason phrase as the message.
    - Network failures (httpx.TransportError) are NOT wrapped; let them propagate.
    """
    headers = {"Authorization": f"Bearer {token}"} if token else {}
    response = client.request(method, path, params=params, json=json, headers=headers)
    if response.is_success:
        return response.json() if response.content else None
    try:
        error = response.json()["error"]
        raise ApiError(response.status_code, error["code"], error["message"])
    except ValueError, KeyError, TypeError:
        raise ApiError(response.status_code, "http_error", response.reason_phrase) from None
