"""One reusable Client, timeouts, non-2xx responses and transport errors."""

import socket
import time

import httpx
from m07_helpers import Reply, Request, json_reply, serve


def api(request: Request) -> Reply:
    if request.path == "/slow":
        time.sleep(0.3)
        return json_reply({"ok": True})
    if request.path == "/tasks/1":
        return json_reply({"id": 1, "title": "Write docs"})
    error = {"error": {"code": "not_found", "message": "task not found"}}
    return json_reply(error, status=404)


def closed_port() -> int:
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return int(sock.getsockname()[1])


def main() -> None:
    with serve(api) as base_url:
        # A non-2xx answer is a normal response; nothing is raised until you ask for it
        timeout = httpx.Timeout(2.0, connect=0.5)
        with httpx.Client(base_url=base_url, timeout=timeout) as client:
            response = client.get("/tasks/42")
            print("status:", response.status_code, "success:", response.is_success)
            try:
                response.raise_for_status()
            except httpx.HTTPStatusError as exc:
                print("raised:", type(exc).__name__, exc.response.json()["error"]["code"])

            # timeout= per request overrides the client default
            try:
                client.get("/slow", timeout=0.05)
            except httpx.ReadTimeout as exc:
                print("raised:", type(exc).__name__, isinstance(exc, httpx.TransportError))

        # Nothing listens on this port: the request never produced a response
        try:
            httpx.get(f"http://127.0.0.1:{closed_port()}/", timeout=1.0)
        except httpx.ConnectError as exc:
            print("raised:", type(exc).__name__, isinstance(exc, httpx.TransportError))

        # Connection reuse: the server sees which client port each request came from
        ports: set[int] = set()

        def tracking(request: Request) -> Reply:
            ports.add(request.client_port)
            return json_reply({})

        with serve(tracking) as tracked_url:
            with httpx.Client(base_url=tracked_url) as client:
                for _ in range(3):
                    client.get("/")
            print("one Client, 3 requests, connections:", len(ports))
            ports.clear()
            for _ in range(3):
                httpx.get(f"{tracked_url}/")
            print("httpx.get x3, connections:", len(ports))


if __name__ == "__main__":
    main()
