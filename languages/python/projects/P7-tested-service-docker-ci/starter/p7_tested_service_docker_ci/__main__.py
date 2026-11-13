import logging
import os
import sys

import uvicorn

from .app import create_app
from .config import ConfigError, load_config


def main() -> None:
    try:
        config = load_config(os.environ)
    except ConfigError as err:
        print(f"error: {err}", file=sys.stderr)
        raise SystemExit(2) from err
    logging.basicConfig(level=config.logging_level, format="%(levelname)s %(name)s: %(message)s")
    host, port = config.host, config.port
    if addr := os.environ.get("ADDR"):  # `host:port` override, same convention as the other lessons
        host, _, raw_port = addr.rpartition(":")
        port = int(raw_port)
    uvicorn.run(create_app(config), host=host, port=port, log_config=None)


if __name__ == "__main__":
    main()
