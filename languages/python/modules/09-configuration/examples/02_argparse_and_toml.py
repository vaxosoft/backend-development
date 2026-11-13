"""Command-line flags with argparse and configuration files with tomllib."""

import argparse
import tempfile
import tomllib
from pathlib import Path

CONFIG_TEXT = """
title = "Task Tracker"

[server]
host = "0.0.0.0"
port = 8080

[logging]
level = "info"
sinks = ["stderr", "file"]
"""


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="tracker", exit_on_error=False)
    parser.add_argument("--host", default=None)
    parser.add_argument("--port", type=int, default=None)
    parser.add_argument("--debug", action=argparse.BooleanOptionalAction, default=None)
    return parser


def main() -> None:
    # argparse: pass an explicit list in tests; the default is sys.argv[1:]
    parser = build_parser()
    print(parser.parse_args(["--port", "9000", "--debug"]))
    print(parser.parse_args([]))  # None means "not given", so a lower layer may decide

    # Bad input raises ArgumentError instead of exiting only because exit_on_error=False
    for argv in (["--port", "x"], ["--unknown"]):
        try:
            parser.parse_args(argv)
        except argparse.ArgumentError as exc:
            print("ArgumentError:", exc)

    # tomllib parses text with loads() and binary file objects with load()
    data = tomllib.loads(CONFIG_TEXT)
    print(data["server"], data["logging"]["sinks"])

    with tempfile.TemporaryDirectory() as tmp:
        path = Path(tmp) / "config.toml"
        path.write_text(CONFIG_TEXT, encoding="utf-8")
        with path.open("rb") as handle:  # TOML is always UTF-8: open in binary mode
            print(tomllib.load(handle)["title"])

    try:
        tomllib.loads("port = ")
    except tomllib.TOMLDecodeError as exc:
        print("TOMLDecodeError:", exc)


if __name__ == "__main__":
    main()
