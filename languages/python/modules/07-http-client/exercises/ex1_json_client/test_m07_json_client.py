import json

import httpx
import pytest
from m07_json_client import ApiError, request_json


def make_client(handler: httpx.MockTransport) -> httpx.Client:
    return httpx.Client(transport=handler, base_url="http://api.test")


def test_returns_parsed_json_for_success() -> None:
    client = make_client(httpx.MockTransport(lambda r: httpx.Response(200, json={"id": 1})))

    assert request_json(client, "GET", "/tasks/1") == {"id": 1}


def test_returns_none_for_empty_body() -> None:
    client = make_client(httpx.MockTransport(lambda r: httpx.Response(204)))

    assert request_json(client, "DELETE", "/tasks/1") is None


def test_sends_bearer_token_query_params_and_json_body() -> None:
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(201, json={"ok": True})

    client = make_client(httpx.MockTransport(handler))
    request_json(
        client, "POST", "/tasks", token="abc", params={"limit": 5}, json={"title": "Write docs"}
    )

    request = seen[0]
    assert request.headers["Authorization"] == "Bearer abc"
    assert request.url.params["limit"] == "5"
    assert json.loads(request.content) == {"title": "Write docs"}


def test_no_authorization_header_without_token() -> None:
    seen: list[httpx.Request] = []

    def handler(request: httpx.Request) -> httpx.Response:
        seen.append(request)
        return httpx.Response(200, json=[])

    request_json(make_client(httpx.MockTransport(handler)), "GET", "/tasks")

    assert "Authorization" not in seen[0].headers


def test_error_envelope_becomes_api_error() -> None:
    body = {"error": {"code": "validation_failed", "message": "invalid task", "fields": {}}}
    client = make_client(httpx.MockTransport(lambda r: httpx.Response(422, json=body)))

    with pytest.raises(ApiError) as info:
        request_json(client, "POST", "/tasks", json={})

    assert (info.value.status, info.value.code, info.value.message) == (
        422,
        "validation_failed",
        "invalid task",
    )


@pytest.mark.parametrize("content", [b"<html>Bad gateway</html>", b'{"detail": "nope"}', b"[1, 2]"])
def test_non_envelope_error_bodies_use_a_generic_code(content: bytes) -> None:
    client = make_client(httpx.MockTransport(lambda r: httpx.Response(502, content=content)))

    with pytest.raises(ApiError) as info:
        request_json(client, "GET", "/tasks")

    assert info.value.status == 502
    assert info.value.code == "http_error"
    assert info.value.message == "Bad Gateway"


def test_transport_errors_are_not_wrapped() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("refused", request=request)

    with pytest.raises(httpx.ConnectError):
        request_json(make_client(httpx.MockTransport(handler)), "GET", "/tasks")
