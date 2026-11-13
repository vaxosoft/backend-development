import threading
import time

import httpx
import pytest
from p4_api_aggregator.app import create_app
from p4_api_aggregator.config import Config
from p4_fake_upstream import Behaviour, FakeUpstream, MakeClient, serve, unused_port

Upstream = tuple[FakeUpstream, str]


def ids_param(*ids: int) -> dict[str, str]:
    return {"ids": ",".join(str(i) for i in ids)}


# --- request validation ---------------------------------------------------------------


@pytest.mark.parametrize(
    "query",
    [
        pytest.param({}, id="missing"),
        pytest.param({"ids": ""}, id="empty"),
        pytest.param({"ids": "1,two,3"}, id="non-numeric"),
        pytest.param({"ids": "1,,3"}, id="empty-entry"),
        pytest.param({"ids": "-1"}, id="negative"),
        pytest.param({"ids": ",".join(str(i) for i in range(51))}, id="more-than-50"),
    ],
)
def test_invalid_ids_are_rejected_with_400(make_client: MakeClient, query: dict[str, str]) -> None:
    response = make_client().get("/aggregate", params=query)

    assert response.status_code == 400
    assert response.json()["error"]["code"] == "bad_request"
    assert response.headers["content-type"].startswith("application/json")


def test_exactly_fifty_ids_are_accepted(make_client: MakeClient) -> None:
    response = make_client().get("/aggregate", params=ids_param(*range(50)))

    assert response.status_code == 200
    assert len(response.json()["results"]) == 50


# --- aggregation and ordering ---------------------------------------------------------


def test_returns_upstream_json_for_each_id(make_client: MakeClient) -> None:
    response = make_client().get("/aggregate", params=ids_param(1, 2))

    assert response.status_code == 200
    assert response.json() == {
        "results": [
            {"id": 1, "data": {"id": 1, "name": "item-1"}},
            {"id": 2, "data": {"id": 2, "name": "item-2"}},
        ],
        "errors": [],
    }


def test_results_keep_request_order_even_when_upstream_answers_out_of_order(
    upstream: Upstream, make_client: MakeClient
) -> None:
    fake, _ = upstream
    fake.behaviour[3] = Behaviour(delay=0.15)

    response = make_client().get("/aggregate", params=ids_param(3, 1, 2))

    assert [r["id"] for r in response.json()["results"]] == [3, 1, 2]


def test_partial_failure_is_still_200_with_errors_listed(
    upstream: Upstream, make_client: MakeClient
) -> None:
    fake, _ = upstream
    fake.behaviour[2] = Behaviour(status=500)

    response = make_client().get("/aggregate", params=ids_param(1, 2, 3))

    assert response.status_code == 200
    body = response.json()
    assert [r["id"] for r in body["results"]] == [1, 3]
    assert body["errors"] == [{"id": 2, "error": "upstream status 500"}]


# --- concurrency ----------------------------------------------------------------------


def test_calls_run_concurrently_but_never_exceed_max_concurrency(
    upstream: Upstream, make_client: MakeClient
) -> None:
    fake, _ = upstream
    fake.default_delay = 0.1

    response = make_client(max_concurrency=3).get("/aggregate", params=ids_param(*range(1, 10)))

    assert len(response.json()["results"]) == 9
    assert fake.max_in_flight == 3


def test_max_concurrency_one_runs_calls_one_at_a_time(
    upstream: Upstream, make_client: MakeClient
) -> None:
    fake, _ = upstream
    fake.default_delay = 0.01

    make_client(max_concurrency=1).get("/aggregate", params=ids_param(1, 2, 3))

    assert fake.max_in_flight == 1


# --- retries --------------------------------------------------------------------------


def test_5xx_is_retried_until_it_succeeds(upstream: Upstream, make_client: MakeClient) -> None:
    fake, _ = upstream
    fake.behaviour[1] = Behaviour(fail_first=2)

    body = make_client().get("/aggregate", params=ids_param(1)).json()

    assert body["errors"] == []
    assert body["results"][0]["data"]["name"] == "item-1"
    assert fake.calls[1] == 3


def test_gives_up_after_two_retries(upstream: Upstream, make_client: MakeClient) -> None:
    fake, _ = upstream
    fake.behaviour[1] = Behaviour(status=503)

    body = make_client().get("/aggregate", params=ids_param(1)).json()

    assert body["errors"] == [{"id": 1, "error": "upstream status 503"}]
    assert fake.calls[1] == 3


def test_4xx_is_never_retried(upstream: Upstream, make_client: MakeClient) -> None:
    fake, _ = upstream
    fake.behaviour[1] = Behaviour(status=404)

    body = make_client().get("/aggregate", params=ids_param(1)).json()

    assert body["errors"] == [{"id": 1, "error": "upstream status 404"}]
    assert fake.calls[1] == 1


def test_timeouts_are_retried_and_reported(upstream: Upstream, make_client: MakeClient) -> None:
    fake, _ = upstream
    fake.behaviour[1] = Behaviour(delay=0.5)

    body = make_client(call_timeout=0.05).get("/aggregate", params=ids_param(1)).json()

    assert body["errors"] == [{"id": 1, "error": "upstream timeout"}]
    assert fake.calls[1] == 3


def test_unreachable_upstream_is_reported_per_id(make_client: MakeClient) -> None:
    client = make_client(upstream_url=f"http://127.0.0.1:{unused_port()}")

    body = client.get("/aggregate", params=ids_param(1, 2)).json()

    assert body["results"] == []
    assert body["errors"] == [
        {"id": 1, "error": "upstream unreachable"},
        {"id": 2, "error": "upstream unreachable"},
    ]


def test_backoff_doubles_between_retries(upstream: Upstream, make_client: MakeClient) -> None:
    fake, _ = upstream
    fake.behaviour[1] = Behaviour(status=500)
    client = make_client(backoff_base=0.05)

    started = time.monotonic()
    client.get("/aggregate", params=ids_param(1))

    assert time.monotonic() - started >= 0.05 + 0.10


# --- deadline and cancellation --------------------------------------------------------


def test_overall_deadline_cancels_slow_calls_and_keeps_finished_ones(
    upstream: Upstream, make_client: MakeClient
) -> None:
    fake, _ = upstream
    fake.behaviour[2] = Behaviour(delay=5)
    client = make_client(overall_deadline=0.3, call_timeout=10)

    started = time.monotonic()
    body = client.get("/aggregate", params=ids_param(1, 2)).json()

    assert time.monotonic() - started < 3
    assert [r["id"] for r in body["results"]] == [1]
    assert body["errors"] == [{"id": 2, "error": "deadline exceeded"}]


def test_client_disconnect_cancels_outstanding_upstream_calls(upstream: Upstream) -> None:
    fake, url = upstream
    fake.default_delay = 10
    config = Config(upstream_url=url, call_timeout=30, overall_deadline=30)

    with serve(create_app(config)) as aggregator_url:
        with pytest.raises(httpx.ReadTimeout), httpx.Client(timeout=0.3) as caller:
            caller.get(f"{aggregator_url}/aggregate", params=ids_param(1, 2))

        deadline = time.monotonic() + 3
        while fake.abandoned < 2 and time.monotonic() < deadline:
            threading.Event().wait(0.01)

    assert fake.abandoned == 2
    assert fake.calls[1] == 1  # cancelled, not retried
