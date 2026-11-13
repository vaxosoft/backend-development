import re

import pytest
from fastapi.testclient import TestClient
from m02_middleware import create_app

ORIGIN = "https://app.example.com"


@pytest.fixture
def client() -> TestClient:
    return TestClient(create_app())


def test_generates_request_id_when_absent(client: TestClient) -> None:
    response = client.get("/ok")
    assert re.fullmatch(r"[0-9a-f]{32}", response.headers["x-request-id"])


def test_ids_are_unique_per_request(client: TestClient) -> None:
    first = client.get("/ok").headers["x-request-id"]
    second = client.get("/ok").headers["x-request-id"]
    assert first != second


def test_echoes_valid_incoming_id_and_exposes_it_to_handlers(client: TestClient) -> None:
    response = client.get("/whoami", headers={"X-Request-ID": "abc-123_X"})
    assert response.headers["x-request-id"] == "abc-123_X"
    assert response.json() == {"request_id": "abc-123_X"}


@pytest.mark.parametrize("bad", ["has space", "x" * 65, "semi;colon", "new\tline"])
def test_replaces_invalid_incoming_id(client: TestClient, bad: str) -> None:
    returned = client.get("/ok", headers={"X-Request-ID": bad}).headers["x-request-id"]
    assert returned != bad
    assert re.fullmatch(r"[0-9a-f]{32}", returned)


def test_id_is_present_on_404(client: TestClient) -> None:
    response = client.get("/missing", headers={"X-Request-ID": "r-404"})
    assert response.status_code == 404
    assert response.headers["x-request-id"] == "r-404"


def test_timing_header_is_a_non_negative_number(client: TestClient) -> None:
    assert float(client.get("/ok").headers["x-process-time-ms"]) >= 0


def test_unhandled_error_becomes_500_envelope_without_leaking(client: TestClient) -> None:
    response = client.get("/boom", headers={"X-Request-ID": "r-500"})
    assert response.status_code == 500
    assert response.json() == {"error": {"code": "internal", "message": "internal server error"}}
    assert "secret" not in response.text
    assert response.headers["x-request-id"] == "r-500"


def test_server_keeps_serving_after_an_error(client: TestClient) -> None:
    assert client.get("/boom").status_code == 500
    assert client.get("/ok").json() == {"status": "ok"}


def test_cors_preflight_for_allowed_origin(client: TestClient) -> None:
    response = client.options(
        "/ok",
        headers={
            "Origin": ORIGIN,
            "Access-Control-Request-Method": "POST",
            "Access-Control-Request-Headers": "content-type",
        },
    )
    assert response.status_code == 200
    assert response.headers["access-control-allow-origin"] == ORIGIN
    assert "POST" in response.headers["access-control-allow-methods"]


def test_cors_actual_request_exposes_request_id(client: TestClient) -> None:
    response = client.get("/ok", headers={"Origin": ORIGIN})
    assert response.headers["access-control-allow-origin"] == ORIGIN
    assert "x-request-id" in response.headers["access-control-expose-headers"].lower()


def test_cors_is_applied_to_500_responses(client: TestClient) -> None:
    response = client.get("/boom", headers={"Origin": ORIGIN})
    assert response.status_code == 500
    assert response.headers["access-control-allow-origin"] == ORIGIN


def test_cors_ignores_other_origins(client: TestClient) -> None:
    response = client.get("/ok", headers={"Origin": "https://evil.example"})
    assert response.status_code == 200
    assert "access-control-allow-origin" not in response.headers
