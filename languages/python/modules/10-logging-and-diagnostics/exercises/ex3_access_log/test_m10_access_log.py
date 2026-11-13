import logging
from itertools import count

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient
from m10_access_log import AccessLogMiddleware

LOGGER_NAME = "tracker.test.access"


class StepClock:
    """Every reading advances time by 250 ms, so one request lasts exactly 0.25 s."""

    def __init__(self) -> None:
        self.now = 0.0

    def __call__(self) -> float:
        value = self.now
        self.now += 0.25
        return value


@pytest.fixture
def client() -> TestClient:
    app = FastAPI()
    ids = (f"gen-{n}" for n in count(1))

    @app.get("/ok")
    def ok() -> dict[str, str]:
        return {"status": "ok"}

    @app.get("/boom")
    def boom() -> None:
        raise RuntimeError("secret internal detail")

    app.add_middleware(
        AccessLogMiddleware,
        logger=logging.getLogger(LOGGER_NAME),
        clock=StepClock(),
        id_factory=lambda: next(ids),
    )
    return TestClient(app)


def access_records(caplog: pytest.LogCaptureFixture) -> list[logging.LogRecord]:
    return [r for r in caplog.records if r.name == LOGGER_NAME and r.getMessage() == "request"]


def test_logs_one_record_with_request_fields(
    client: TestClient, caplog: pytest.LogCaptureFixture
) -> None:
    caplog.set_level(logging.INFO, logger=LOGGER_NAME)

    response = client.get("/ok")

    assert response.status_code == 200
    [record] = access_records(caplog)
    assert record.levelno == logging.INFO
    fields = {k: record.__dict__[k] for k in ("request_id", "method", "path", "status")}
    assert fields == {"request_id": "gen-1", "method": "GET", "path": "/ok", "status": 200}
    assert record.__dict__["duration_ms"] == 250.0


def test_request_id_is_echoed_when_valid_and_replaced_when_not(
    client: TestClient, caplog: pytest.LogCaptureFixture
) -> None:
    caplog.set_level(logging.INFO, logger=LOGGER_NAME)

    echoed = client.get("/ok", headers={"X-Request-ID": "client-42"})
    replaced = client.get("/ok", headers={"X-Request-ID": "bad id\t;"})

    assert echoed.headers["X-Request-ID"] == "client-42"
    assert replaced.headers["X-Request-ID"] == "gen-1"
    assert [r.__dict__["request_id"] for r in access_records(caplog)] == ["client-42", "gen-1"]


def test_unhandled_error_becomes_a_500_envelope_and_a_logged_traceback(
    client: TestClient, caplog: pytest.LogCaptureFixture
) -> None:
    caplog.set_level(logging.INFO, logger=LOGGER_NAME)

    response = client.get("/boom")

    assert response.status_code == 500
    assert response.json() == {"error": {"code": "internal", "message": "internal server error"}}
    assert "secret" not in response.text
    assert response.headers["X-Request-ID"] == "gen-1"

    [error] = [r for r in caplog.records if r.levelno == logging.ERROR]
    assert error.exc_info is not None
    assert error.__dict__["request_id"] == "gen-1"
    [record] = access_records(caplog)
    assert record.__dict__["status"] == 500


def test_the_server_keeps_serving_after_a_failure(client: TestClient) -> None:
    assert client.get("/boom").status_code == 500

    assert client.get("/ok").status_code == 200
