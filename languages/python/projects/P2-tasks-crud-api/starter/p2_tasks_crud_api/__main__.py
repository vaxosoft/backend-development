import os

import uvicorn

from .app import Config, create_app


def parse_addr(value: str) -> tuple[str, int]:
    host, _, port = value.rpartition(":")
    return host or "127.0.0.1", int(port)


def main() -> None:
    host, port = parse_addr(os.environ.get("ADDR", "127.0.0.1:8080"))
    app = create_app(Config(db_path=os.environ.get("DB_PATH", "tasks.db")))
    # uvicorn handles SIGINT/SIGTERM itself: it stops accepting and drains in-flight requests.
    uvicorn.run(app, host=host, port=port, timeout_graceful_shutdown=10)


if __name__ == "__main__":
    main()
