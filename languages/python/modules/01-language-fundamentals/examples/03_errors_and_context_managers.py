"""Exceptions: a small hierarchy, exception chaining and context managers."""

from collections.abc import Iterator
from contextlib import contextmanager


class AppError(Exception):
    """Base class so callers can catch every domain error at once."""


class NotFoundError(AppError):
    def __init__(self, kind: str, ident: int) -> None:
        super().__init__(f"{kind} {ident} not found")
        self.kind = kind
        self.ident = ident


class ValidationError(AppError):
    def __init__(self, fields: dict[str, str]) -> None:
        super().__init__("invalid input")
        self.fields = fields


def parse_id(raw: str) -> int:
    try:
        return int(raw)
    except ValueError as err:
        raise ValidationError({"id": "must be an integer"}) from err


def find(tasks: dict[int, str], raw_id: str) -> str:
    ident = parse_id(raw_id)
    try:
        return tasks[ident]
    except KeyError:
        raise NotFoundError("task", ident) from None


@contextmanager
def logged(step: str) -> Iterator[None]:
    """Setup before `yield`, cleanup in `finally` (runs even on exceptions)."""
    print(f"[{step}] start")
    try:
        yield
    except AppError as err:
        print(f"[{step}] failed: {err}")
        raise
    finally:
        print(f"[{step}] end")


def main() -> None:
    tasks = {1: "Write docs"}

    for raw in ("1", "2", "x"):
        try:
            print("found:", find(tasks, raw))
        except NotFoundError as err:
            print("not found:", err, "->", err.kind, err.ident)
        except ValidationError as err:
            print("validation:", err.fields, "| cause:", repr(err.__cause__))

    # try/else/finally: `else` runs only when nothing was raised.
    try:
        value = find(tasks, "1")
    except AppError:
        print("unexpected")
    else:
        print("else branch:", value)
    finally:
        print("finally always runs")

    try:
        with logged("lookup"):
            find(tasks, "99")
    except AppError as err:
        print("caught outside:", type(err).__name__)

    # Exception groups collect several errors (used by asyncio.TaskGroup, module 06).
    try:
        raise ExceptionGroup("batch", [ValidationError({"a": "bad"}), NotFoundError("task", 7)])
    except* ValidationError as group:
        print("group validation:", [type(e).__name__ for e in group.exceptions])
    except* NotFoundError as group:
        print("group not found:", [str(e) for e in group.exceptions])


if __name__ == "__main__":
    main()
