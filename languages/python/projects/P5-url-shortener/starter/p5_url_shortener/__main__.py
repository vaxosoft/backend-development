import os

import uvicorn

from .app import create_app
from .config import Config


def parse_addr(value: str) -> tuple[str, int]:
    host, _, port = value.rpartition(":")
    return host or "127.0.0.1", int(port)


def main() -> None:
    host, port = parse_addr(os.environ.get("ADDR", "127.0.0.1:8080"))
    # access_log is our own JSON line per request, so silence uvicorn's
    uvicorn.run(create_app(Config.from_env()), host=host, port=port, access_log=False)


if __name__ == "__main__":
    main()
