import os

import uvicorn

from .app import create_app
from .config import Config


def main() -> None:
    host, _, port = os.environ.get("ADDR", "127.0.0.1:8080").rpartition(":")
    uvicorn.run(create_app(Config.from_env()), host=host, port=int(port))


if __name__ == "__main__":
    main()
