"""Exercise 2: export and import tasks as CSV."""

from dataclasses import dataclass
from datetime import date

COLUMNS = ["id", "title", "status", "due_date", "tags"]


@dataclass(frozen=True, slots=True)
class Task:
    id: int
    title: str
    status: str
    due_date: date | None
    tags: tuple[str, ...]


def tasks_to_csv(tasks: list[Task]) -> str:
    """Header row `id,title,status,due_date,tags`, then one row per task.

    `due_date` is ISO or empty, `tags` are joined with ";". Lines end with "\\n".
    Titles may contain commas, quotes and newlines: the output must survive a round trip.
    """
    raise NotImplementedError


def tasks_from_csv(text: str) -> list[Task]:
    """Inverse of `tasks_to_csv`.

    Raise ValueError if a column from COLUMNS is missing (message names it) or a value is
    invalid (non-integer id, malformed date). Extra columns are ignored.
    """
    raise NotImplementedError
