import httpx
import pytest
from m07_retry import request_with_retry


class Script:
    """A MockTransport handler that plays back a list of responses or exceptions."""

    def __init__(self, *steps: int | Exception) -> None:
        self.steps = list(steps)
        self.calls = 0

    def __call__(self, request: httpx.Request) -> httpx.Response:
        step = self.steps[min(self.calls, len(self.steps) - 1)]
        self.calls += 1
        if isinstance(step, Exception):
            raise step
        return httpx.Response(step, json={"status": step})


def run(
    script: Script, method: str = "GET", retries: int = 2
) -> tuple[httpx.Response | None, list[float], Exception | None]:
    delays: list[float] = []
    client = httpx.Client(transport=httpx.MockTransport(script), base_url="http://api.test")
    try:
        response = request_with_retry(
            client, method, "/x", retries=retries, base_delay=0.1, sleep=delays.append
        )
    except (httpx.TransportError, httpx.HTTPStatusError) as exc:
        return None, delays, exc
    return response, delays, None


def test_success_needs_no_sleep() -> None:
    script = Script(200)

    response, delays, _ = run(script)

    assert response is not None
    assert response.status_code == 200
    assert (script.calls, delays) == (1, [])


def test_retries_5xx_with_doubling_delays_until_success() -> None:
    script = Script(503, 500, 200)

    response, delays, _ = run(script)

    assert response is not None
    assert response.status_code == 200
    assert script.calls == 3
    assert delays == [0.1, 0.2]


def test_client_errors_are_never_retried() -> None:
    script = Script(404, 200)

    response, delays, _ = run(script)

    assert response is not None
    assert response.status_code == 404
    assert (script.calls, delays) == (1, [])


def test_exhausted_5xx_raises_http_status_error() -> None:
    script = Script(502)

    response, delays, error = run(script, retries=2)

    assert response is None
    assert isinstance(error, httpx.HTTPStatusError)
    assert error.response.status_code == 502
    assert script.calls == 3
    assert delays == [0.1, 0.2]


def test_transport_errors_are_retried() -> None:
    script = Script(httpx.ConnectError("refused"), httpx.ReadTimeout("slow"), 200)

    response, delays, _ = run(script)

    assert response is not None
    assert response.status_code == 200
    assert delays == [0.1, 0.2]


def test_exhausted_transport_error_is_reraised() -> None:
    script = Script(httpx.ConnectError("refused"))

    response, delays, error = run(script, retries=1)

    assert response is None
    assert isinstance(error, httpx.ConnectError)
    assert (script.calls, delays) == (2, [0.1])


@pytest.mark.parametrize("method", ["POST", "PATCH"])
def test_non_idempotent_methods_are_sent_once(method: str) -> None:
    script = Script(503, 200)

    response, delays, _ = run(script, method=method)

    assert response is not None
    assert response.status_code == 503
    assert (script.calls, delays) == (1, [])


def test_zero_retries_means_one_attempt() -> None:
    script = Script(500)

    _, delays, error = run(script, retries=0)

    assert isinstance(error, httpx.HTTPStatusError)
    assert (script.calls, delays) == (1, [])
