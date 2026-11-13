"""A tiny loopback HTTP server so the examples run offline and deterministically."""

import json
import threading
from collections.abc import Callable, Iterator
from contextlib import contextmanager
from dataclasses import dataclass, field
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any


@dataclass(slots=True)
class Request:
    method: str
    path: str
    headers: dict[str, str]
    body: bytes
    client_port: int


@dataclass(slots=True)
class Reply:
    status: int = 200
    body: bytes = b""
    headers: dict[str, str] = field(default_factory=dict)


def json_reply(payload: object, status: int = 200) -> Reply:
    return Reply(status, json.dumps(payload).encode(), {"Content-Type": "application/json"})


Handler = Callable[[Request], Reply]


@contextmanager
def serve(handler: Handler) -> Iterator[str]:
    """Run `handler` on 127.0.0.1 on a free port and yield the base URL."""

    class _Handler(BaseHTTPRequestHandler):
        protocol_version = "HTTP/1.1"  # keep-alive, so connection reuse is observable

        def _handle(self) -> None:
            length = int(self.headers.get("Content-Length", 0))
            request = Request(
                self.command,
                self.path,
                {k.lower(): v for k, v in self.headers.items()},
                self.rfile.read(length),
                self.client_address[1],
            )
            reply = handler(request)
            self.send_response(reply.status)
            for name, value in reply.headers.items():
                self.send_header(name, value)
            self.send_header("Content-Length", str(len(reply.body)))
            self.end_headers()
            self.wfile.write(reply.body)

        do_GET = do_POST = do_PUT = do_PATCH = do_DELETE = _handle

        def log_message(self, format: str, *args: Any) -> None:
            pass

    class _Server(ThreadingHTTPServer):
        def handle_error(self, request: Any, client_address: Any) -> None:
            pass  # clients that time out and hang up are expected in these examples

    server = _Server(("127.0.0.1", 0), _Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield f"http://127.0.0.1:{server.server_address[1]}"
    finally:
        server.shutdown()
        server.server_close()
        thread.join()
