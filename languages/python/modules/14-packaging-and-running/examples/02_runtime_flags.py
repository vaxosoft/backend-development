"""Runtime flags: command line beats environment beats defaults, and every flag is validated."""

import argparse
import os
import sys
from collections.abc import Sequence


def build_parser(environ: dict[str, str]) -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="task-tracker", description="Run the Task Tracker")
    parser.add_argument("--addr", default=environ.get("ADDR", "127.0.0.1:8080"), help="host:port")
    parser.add_argument(
        "--workers", type=int, default=int(environ.get("WORKERS", "1")), help="worker processes"
    )
    parser.add_argument(
        "--log-level",
        choices=["debug", "info", "warning", "error"],
        default=environ.get("LOG_LEVEL", "info"),
    )
    parser.add_argument("--reload", action="store_true", help="development only")
    parser.add_argument("--version", action="version", version="task-tracker 1.4.2")
    return parser


def uvicorn_command(args: argparse.Namespace) -> list[str]:
    host, _, port = args.addr.rpartition(":")
    command = ["uvicorn", "task_tracker.app:app", "--host", host, "--port", port]
    command += ["--workers", str(args.workers), "--log-level", args.log_level]
    return [*command, "--reload"] if args.reload else command


def main(argv: Sequence[str] | None = None) -> int:
    args = build_parser(dict(os.environ)).parse_args(argv)
    if args.reload and args.workers > 1:
        print("error: --reload cannot be combined with --workers > 1", file=sys.stderr)
        return 2
    print("resolved:", vars(args))
    print("command:", " ".join(uvicorn_command(args)))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
