"""A server with only the standard library: shows what a framework hides.

Run:  python 01_stdlib_server.py   (ADDR=host:port to change the address)
"""

import json
import os
import signal
import threading
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any
from urllib.parse import parse_qs, urlsplit

DEFAULT_ADDR = "127.0.0.1:8080"


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"  # keep-alive needs an explicit Content-Length on every reply

    def do_GET(self) -> None:
        url = urlsplit(self.path)
        query = parse_qs(url.query)
        if url.path == "/healthz":
            self._json(HTTPStatus.OK, {"status": "ok"})
        elif url.path == "/hello":
            name = query.get("name", ["world"])[0]
            self._text(HTTPStatus.OK, f"Hello, {name}!\n")
        elif url.path.startswith("/hello/"):
            self._text(HTTPStatus.OK, f"Hello, {url.path.removeprefix('/hello/')}!\n")
        else:
            self._json(HTTPStatus.NOT_FOUND, {"error": "not found"})

    def do_POST(self) -> None:
        if self.path != "/echo":
            self._json(HTTPStatus.NOT_FOUND, {"error": "not found"})
            return
        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length)
        try:
            body = json.loads(raw)
        except json.JSONDecodeError:
            self._json(HTTPStatus.BAD_REQUEST, {"error": "malformed JSON"})
            return
        self._json(HTTPStatus.OK, {"you_sent": body})

    def _json(self, status: HTTPStatus, payload: Any) -> None:
        self._send(status, "application/json", json.dumps(payload).encode())

    def _text(self, status: HTTPStatus, text: str) -> None:
        self._send(status, "text/plain; charset=utf-8", text.encode())

    def _send(self, status: HTTPStatus, content_type: str, body: bytes) -> None:
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, format: str, *args: Any) -> None:
        print(f"{self.command} {self.path} -> {args[1]}", flush=True)


def main() -> None:
    host, _, port = os.environ.get("ADDR", DEFAULT_ADDR).rpartition(":")
    server = ThreadingHTTPServer((host, int(port)), Handler)

    stop = threading.Event()
    signal.signal(signal.SIGINT, lambda *_: stop.set())
    signal.signal(signal.SIGTERM, lambda *_: stop.set())

    worker = threading.Thread(target=server.serve_forever)
    worker.start()
    print(f"listening on http://{host}:{port}", flush=True)
    stop.wait()

    print("shutting down", flush=True)
    server.shutdown()  # stops the accept loop; must not run in the serving thread
    server.server_close()
    worker.join()


if __name__ == "__main__":
    main()
