import asyncio
import socket

import httpx
from p1_hello_http.app import Config, create_app
from p1_hello_http.server import make_server, run


async def _wait_until_refused(port: int) -> None:
    for _ in range(500):
        try:
            _, writer = await asyncio.open_connection("127.0.0.1", port)
        except ConnectionRefusedError:
            return
        writer.close()
        await asyncio.sleep(0.01)
    raise AssertionError("the server still accepts connections")


def test_server_timeouts_are_configured() -> None:
    config = make_server(create_app()).config
    assert config.timeout_graceful_shutdown is not None
    assert config.timeout_keep_alive <= 30


def test_in_flight_request_finishes_and_new_connections_are_refused_after_shutdown() -> None:
    async def scenario() -> None:
        started, release, shutdown = asyncio.Event(), asyncio.Event(), asyncio.Event()

        async def gated_sleep(_seconds: float) -> None:
            started.set()
            await release.wait()

        app = create_app(Config(log_sink=lambda _line: None, sleep=gated_sleep))
        sock = socket.create_server(("127.0.0.1", 0))
        port = sock.getsockname()[1]
        serving = asyncio.create_task(run(app, sock, shutdown))

        async with httpx.AsyncClient(base_url=f"http://127.0.0.1:{port}", timeout=2) as client:
            in_flight = asyncio.create_task(client.get("/slow?ms=1"))
            await asyncio.wait_for(started.wait(), 2)

            shutdown.set()
            await asyncio.wait_for(_wait_until_refused(port), 5)
            assert not in_flight.done()
            assert not serving.done()

            release.set()
            response = await asyncio.wait_for(in_flight, 5)

        await asyncio.wait_for(serving, 5)
        assert response.status_code == 200
        assert response.text == "done"

    asyncio.run(scenario())


def test_run_serves_requests_until_shutdown_is_requested() -> None:
    async def scenario() -> None:
        shutdown = asyncio.Event()
        sock = socket.create_server(("127.0.0.1", 0))
        port = sock.getsockname()[1]
        serving = asyncio.create_task(
            run(create_app(Config(log_sink=lambda _l: None)), sock, shutdown)
        )

        async with httpx.AsyncClient(base_url=f"http://127.0.0.1:{port}", timeout=2) as client:
            first = await client.get("/hello/Ada")
            second = await client.get("/healthz")
        assert (first.text, second.json()) == ("Hello, Ada!", {"status": "ok"})
        assert not serving.done()

        shutdown.set()
        await asyncio.wait_for(serving, 5)

    asyncio.run(scenario())
