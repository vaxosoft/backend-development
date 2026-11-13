import pytest
from fastapi.testclient import TestClient
from p1_hello_http.app import MAX_SLOW_MS, Config, create_app


class Harness:
    """A TestClient plus everything the app wrote to its injected log sink and sleep."""

    def __init__(self) -> None:
        self.lines: list[str] = []
        self.sleeps: list[float] = []
        self.client = TestClient(create_app(Config(log_sink=self.lines.append, sleep=self._sleep)))

    async def _sleep(self, seconds: float) -> None:
        self.sleeps.append(seconds)

    def last_log(self) -> dict[str, str]:
        assert self.lines, "no access log line was written"
        return dict(part.split("=", 1) for part in self.lines[-1].split())


@pytest.fixture
def h() -> Harness:
    return Harness()


def assert_envelope(body: dict[str, dict[str, str]], code: str) -> None:
    assert body["error"]["code"] == code
    assert body["error"]["message"]


# --- routes ---


def test_healthz_returns_ok_json(h: Harness) -> None:
    r = h.client.get("/healthz")
    assert r.status_code == 200
    assert r.headers["content-type"].startswith("application/json")
    assert r.json() == {"status": "ok"}


def test_hello_greets_the_name_as_plain_text(h: Harness) -> None:
    r = h.client.get("/hello/Ada")
    assert r.status_code == 200
    assert r.headers["content-type"].startswith("text/plain")
    assert r.text == "Hello, Ada!"


def test_hello_without_a_name_greets_the_world(h: Harness) -> None:
    r = h.client.get("/hello")
    assert r.status_code == 200
    assert r.text == "Hello, world!"


def test_hello_decodes_percent_encoded_names(h: Harness) -> None:
    assert h.client.get("/hello/Ana%20Maria").text == "Hello, Ana Maria!"


def test_unknown_route_returns_not_found_envelope(h: Harness) -> None:
    r = h.client.get("/nope")
    assert r.status_code == 404
    assert_envelope(r.json(), "not_found")


def test_slow_waits_the_requested_milliseconds_then_says_done(h: Harness) -> None:
    r = h.client.get("/slow?ms=250")
    assert r.status_code == 200
    assert r.text == "done"
    assert h.sleeps == [0.25]


def test_slow_accepts_the_maximum_of_2000_ms(h: Harness) -> None:
    assert h.client.get(f"/slow?ms={MAX_SLOW_MS}").status_code == 200
    assert h.sleeps == [2.0]


@pytest.mark.parametrize("ms", ["2001", "-1", "abc"])
def test_slow_rejects_bad_ms_with_a_400_envelope(h: Harness, ms: str) -> None:
    r = h.client.get("/slow", params={"ms": ms})
    assert r.status_code == 400
    assert_envelope(r.json(), "bad_request")
    assert h.sleeps == []


def test_panic_returns_a_500_envelope_without_details(h: Harness) -> None:
    r = h.client.get("/panic")
    assert r.status_code == 500
    assert r.headers["content-type"].startswith("application/json")
    assert_envelope(r.json(), "internal")
    assert "boom" not in r.text
    assert "Traceback" not in r.text


def test_server_keeps_serving_after_a_panic(h: Harness) -> None:
    h.client.get("/panic")
    h.client.get("/panic")
    assert h.client.get("/healthz").json() == {"status": "ok"}


# --- request id ---


def test_request_id_is_generated_when_missing(h: Harness) -> None:
    first = h.client.get("/healthz").headers["x-request-id"]
    second = h.client.get("/healthz").headers["x-request-id"]
    assert first
    assert first != second


def test_request_id_from_the_client_is_echoed(h: Harness) -> None:
    r = h.client.get("/healthz", headers={"X-Request-ID": "trace-42.a_b"})
    assert r.headers["x-request-id"] == "trace-42.a_b"


@pytest.mark.parametrize("bad", ["x" * 65, "has space", "semi;colon"])
def test_unsafe_request_id_is_replaced(h: Harness, bad: str) -> None:
    rid = h.client.get("/healthz", headers={"X-Request-ID": bad}).headers["x-request-id"]
    assert rid != bad
    assert rid


def test_request_id_is_present_on_error_responses(h: Harness) -> None:
    r = h.client.get("/panic", headers={"X-Request-ID": "abc"})
    assert r.status_code == 500
    assert r.headers["x-request-id"] == "abc"


# --- access log ---


def test_access_log_writes_one_line_per_request(h: Harness) -> None:
    h.client.get("/healthz")
    h.client.get("/hello/x")
    assert len(h.lines) == 2


def test_access_log_line_has_method_path_status_and_duration(h: Harness) -> None:
    h.client.get("/hello/Ada")
    entry = h.last_log()
    assert entry["method"] == "GET"
    assert entry["path"] == "/hello/Ada"
    assert entry["status"] == "200"
    assert float(entry["duration_ms"]) >= 0


def test_access_log_includes_the_request_id(h: Harness) -> None:
    h.client.get("/healthz", headers={"X-Request-ID": "rid-1"})
    assert h.last_log()["request_id"] == "rid-1"


def test_access_log_records_status_500_for_a_panic(h: Harness) -> None:
    h.client.get("/panic")
    assert h.last_log()["status"] == "500"


def test_access_log_cannot_be_split_by_a_newline_in_the_path(h: Harness) -> None:
    h.client.get("/hello/a%0Aforged")
    assert len(h.lines) == 1
    assert "\n" not in h.lines[0]
