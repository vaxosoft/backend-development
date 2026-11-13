"""Acceptance tests for P6. Each test starts the real app, lifespan included, and uses HTTP only."""

import asyncio
import logging
import uuid
from datetime import UTC, datetime, timedelta

import pytest
from p6_support import Server, run_app

# --- submitting and running jobs -------------------------------------------------------------


async def test_post_job_returns_202_with_id_and_location() -> None:
    async with run_app() as server:
        response = await server.client.post(
            "/jobs", json={"type": "reverse", "payload": {"text": "abc"}}
        )

        assert response.status_code == 202
        job_id = response.json()["id"]
        assert response.headers["location"] == f"/jobs/{job_id}"


async def test_reverse_job_succeeds_with_reversed_text() -> None:
    async with run_app() as server:
        job_id = await server.submit("reverse", {"text": "héllo"})

        view = await server.wait_for(job_id, "succeeded", "failed")

        assert view["status"] == "succeeded"
        assert view["result"] == "olléh"
        assert view["attempts"] == 1
        assert view["error"] is None
        assert view["type"] == "reverse"


async def test_sleep_job_succeeds_and_reports_slept_ms() -> None:
    async with run_app() as server:
        job_id = await server.submit("sleep", {"ms": 5})

        view = await server.wait_for(job_id, "succeeded", "failed")

        assert view["status"] == "succeeded"
        assert view["result"] == {"slept_ms": 5}


async def test_job_ids_are_uuid7_and_created_at_is_utc() -> None:
    async with run_app() as server:
        job_id = await server.submit("reverse", {"text": "x"})
        view = await server.job(job_id)

        assert uuid.UUID(job_id).version == 7
        created = datetime.fromisoformat(view["created_at"])
        assert created.utcoffset() is not None
        assert abs((datetime.now(UTC) - created).total_seconds()) < 5


# --- validation -----------------------------------------------------------------------------


async def test_unknown_job_type_is_422_with_envelope() -> None:
    async with run_app() as server:
        response = await server.client.post("/jobs", json={"type": "explode", "payload": {}})

        assert response.status_code == 422
        error = response.json()["error"]
        assert error["code"] == "validation_failed"
        assert "type" in error["fields"]
        assert response.headers["content-type"] == "application/json"


@pytest.mark.parametrize(
    ("job_type", "payload", "field"),
    [
        ("reverse", {}, "payload.text"),
        ("reverse", {"text": 42}, "payload.text"),
        ("sleep", {}, "payload.ms"),
        ("sleep", {"ms": 5001}, "payload.ms"),
        ("sleep", {"ms": -1}, "payload.ms"),
        ("sleep", {"ms": True}, "payload.ms"),
    ],
)
async def test_invalid_payload_is_422_naming_the_field(
    job_type: str, payload: dict[str, object], field: str
) -> None:
    async with run_app() as server:
        response = await server.client.post("/jobs", json={"type": job_type, "payload": payload})

        assert response.status_code == 422
        assert response.json()["error"]["code"] == "validation_failed"
        assert field in response.json()["error"]["fields"]


async def test_malformed_json_is_400_bad_request() -> None:
    async with run_app() as server:
        response = await server.client.post(
            "/jobs", content=b"{not json", headers={"content-type": "application/json"}
        )

        assert response.status_code == 400
        assert response.json()["error"]["code"] == "bad_request"


# --- reading jobs ---------------------------------------------------------------------------


async def test_unknown_job_is_404_with_envelope() -> None:
    async with run_app() as server:
        response = await server.client.get("/jobs/does-not-exist")

        assert response.status_code == 404
        assert response.json()["error"]["code"] == "not_found"


async def test_list_returns_newest_first_with_paging_metadata() -> None:
    async with run_app() as server:
        ids = [await server.submit("reverse", {"text": str(n)}) for n in range(3)]

        response = await server.client.get("/jobs")

        body = response.json()
        assert [item["id"] for item in body["items"]] == ids[::-1]
        assert (body["total"], body["limit"], body["offset"]) == (3, 20, 0)


async def test_list_clamps_limit_and_applies_offset() -> None:
    async with run_app() as server:
        ids = [await server.submit("reverse", {"text": str(n)}) for n in range(3)]

        clamped = (await server.client.get("/jobs", params={"limit": 1000})).json()
        page = (await server.client.get("/jobs", params={"limit": 1, "offset": 1})).json()

        assert clamped["limit"] == 100
        assert [item["id"] for item in page["items"]] == [ids[1]]
        assert page["total"] == 3


async def test_list_with_non_numeric_limit_is_400() -> None:
    async with run_app() as server:
        response = await server.client.get("/jobs", params={"limit": "many"})

        assert response.status_code == 400
        assert response.json()["error"]["code"] == "bad_request"


# --- retries and timeouts -------------------------------------------------------------------


async def test_failing_job_is_retried_until_max_attempts_then_failed() -> None:
    async with run_app(max_attempts=3) as server:
        job_id = await server.submit("fail")

        view = await server.wait_for(job_id, "failed", "succeeded")

        assert view["status"] == "failed"
        assert view["attempts"] == 3
        assert view["error"] == "job failed on purpose"
        assert view["result"] is None


async def test_job_exceeding_the_timeout_fails_with_timeout_error() -> None:
    async with run_app(max_attempts=2, job_timeout=timedelta(milliseconds=20)) as server:
        job_id = await server.submit("sleep", {"ms": 2000})

        view = await server.wait_for(job_id, "failed", "succeeded")

        assert view["status"] == "failed"
        assert view["attempts"] == 2
        assert "timed out" in view["error"]


# --- the pool: workers and the bounded queue ------------------------------------------------


async def test_at_most_workers_jobs_run_at_once() -> None:
    async with run_app(workers=2, queue_capacity=8) as server:
        for _ in range(4):
            await server.submit("sleep", {"ms": 2000})

        async with asyncio.timeout(3):
            while await server.count_status("running") < 2:
                await asyncio.sleep(0.002)
        await asyncio.sleep(0.02)

        assert await server.count_status("running") == 2
        assert await server.count_status("queued") == 2


async def test_full_queue_answers_429_with_retry_after() -> None:
    async with run_app(workers=1, queue_capacity=1) as server:
        running = await server.submit("sleep", {"ms": 2000})
        await server.wait_for(running, "running")
        await server.submit("sleep", {"ms": 2000})  # fills the only queue slot

        response = await server.client.post("/jobs", json={"type": "sleep", "payload": {"ms": 1}})

        assert response.status_code == 429
        assert response.json()["error"]["code"] == "rate_limited"
        assert "retry-after" in response.headers
        assert (await server.client.get("/jobs")).json()["total"] == 2


async def test_rejected_job_is_not_stored_and_pool_recovers() -> None:
    async with run_app(workers=1, queue_capacity=1) as server:
        first = await server.submit("sleep", {"ms": 2000})
        await server.wait_for(first, "running")
        second = await server.submit("reverse", {"text": "ok"})
        rejected = await server.client.post("/jobs", json={"type": "fail", "payload": {}})
        assert rejected.status_code == 429

        assert (await server.client.delete(f"/jobs/{first}")).status_code == 204

        assert (await server.wait_for(second, "succeeded"))["result"] == "ko"


# --- cancellation ---------------------------------------------------------------------------


async def test_cancel_queued_job_marks_it_cancelled_and_it_never_runs() -> None:
    async with run_app(workers=1, queue_capacity=4) as server:
        blocker = await server.submit("sleep", {"ms": 2000})
        await server.wait_for(blocker, "running")
        queued = await server.submit("reverse", {"text": "abc"})

        response = await server.client.delete(f"/jobs/{queued}")
        assert response.status_code == 204
        assert (await server.client.delete(f"/jobs/{blocker}")).status_code == 204
        await asyncio.sleep(0.03)

        view = await server.job(queued)
        assert view["status"] == "cancelled"
        assert view["attempts"] == 0


async def test_cancel_running_job_frees_the_worker() -> None:
    async with run_app(workers=1) as server:
        long_job = await server.submit("sleep", {"ms": 5000})
        await server.wait_for(long_job, "running")
        follow_up = await server.submit("reverse", {"text": "abc"})

        assert (await server.client.delete(f"/jobs/{long_job}")).status_code == 204

        assert (await server.job(long_job))["status"] == "cancelled"
        assert (await server.wait_for(follow_up, "succeeded"))["result"] == "cba"


async def test_cancel_finished_job_is_409_conflict() -> None:
    async with run_app() as server:
        job_id = await server.submit("reverse", {"text": "abc"})
        await server.wait_for(job_id, "succeeded")

        response = await server.client.delete(f"/jobs/{job_id}")

        assert response.status_code == 409
        assert response.json()["error"]["code"] == "conflict"


async def test_cancel_unknown_job_is_404() -> None:
    async with run_app() as server:
        response = await server.client.delete("/jobs/nope")

        assert response.status_code == 404


# --- graceful shutdown ----------------------------------------------------------------------


async def test_shutdown_lets_running_and_queued_jobs_finish() -> None:
    server = await Server.start(workers=1, drain_timeout=timedelta(seconds=3))
    try:
        slow = await server.submit("sleep", {"ms": 60})
        queued = await server.submit("reverse", {"text": "abc"})
        await server.wait_for(slow, "running")

        await server.stop()

        assert (await server.job(slow))["status"] == "succeeded"
        assert (await server.job(queued))["status"] == "succeeded"
    finally:
        await server.aclose()


async def test_shutdown_stops_accepting_new_jobs_with_503() -> None:
    server = await Server.start(workers=1, drain_timeout=timedelta(seconds=3))
    try:
        slow = await server.submit("sleep", {"ms": 150})
        await server.wait_for(slow, "running")

        stopping = asyncio.create_task(server.stop())
        await asyncio.sleep(0.02)
        response = await server.client.post(
            "/jobs", json={"type": "reverse", "payload": {"text": "x"}}
        )
        await stopping

        assert response.status_code == 503
        assert (await server.job(slow))["status"] == "succeeded"
    finally:
        await server.aclose()


async def test_drain_timeout_cancels_jobs_that_do_not_finish_in_time() -> None:
    server = await Server.start(workers=1, drain_timeout=timedelta(milliseconds=30))
    try:
        running = await server.submit("sleep", {"ms": 5000})
        queued = await server.submit("sleep", {"ms": 5000})
        await server.wait_for(running, "running")

        async with asyncio.timeout(2):
            await server.stop()

        assert (await server.job(running))["status"] == "cancelled"
        assert (await server.job(queued))["status"] == "cancelled"
    finally:
        await server.aclose()


# --- diagnostics ----------------------------------------------------------------------------


async def test_finished_job_is_logged_with_job_id_status_and_attempts(
    caplog: pytest.LogCaptureFixture,
) -> None:
    caplog.set_level(logging.INFO, logger="p6_job_worker_pool")
    async with run_app(max_attempts=2) as server:
        job_id = await server.submit("fail")
        await server.wait_for(job_id, "failed")

    records = [r for r in caplog.records if getattr(r, "job_id", None) == job_id]
    assert len(records) == 1
    fields = records[0].__dict__
    assert (fields["status"], fields["attempts"]) == ("failed", 2)
