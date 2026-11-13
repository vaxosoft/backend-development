import os
from pathlib import Path

import uvicorn

from .app import Config, create_app
from .storage import DEFAULT_MAX_SIZE


def parse_addr(value: str) -> tuple[str, int]:
    host, _, port = value.rpartition(":")
    return host or "127.0.0.1", int(port)


def main() -> None:
    host, port = parse_addr(os.environ.get("ADDR", "127.0.0.1:8080"))
    config = Config(
        storage_dir=Path(os.environ.get("STORAGE_DIR", "uploads")),
        max_size=int(os.environ.get("MAX_UPLOAD_BYTES", DEFAULT_MAX_SIZE)),
    )
    # uvicorn handles SIGINT/SIGTERM itself: it stops accepting and drains in-flight requests.
    uvicorn.run(create_app(config), host=host, port=port, timeout_graceful_shutdown=10)


if __name__ == "__main__":
    main()
