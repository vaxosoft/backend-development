"""Pagination, filtering and sorting on a collection.

Run:  python 03_pagination_filtering_sorting.py   (ADDR=host:port to change the address)
"""

from datetime import UTC, date, datetime, timedelta
from typing import Annotated

from fastapi import FastAPI, Query
from m03_errors import install_error_handlers, serve
from m03_tasks import DEFAULT_LIMIT, MAX_LIMIT, SortKey, Status, Task, TaskCreate, TaskStore
from pydantic import BaseModel


class Page(BaseModel):
    items: list[Task]
    total: int
    limit: int
    offset: int


def seeded_store() -> TaskStore:
    """Five tasks created one minute apart, so the sort order is easy to see."""
    ticks = iter(range(100))
    store = TaskStore(
        lambda: datetime(2030, 1, 1, 9, 0, tzinfo=UTC) + timedelta(minutes=next(ticks))
    )
    for title, status, due, tags in [
        ("Write docs", Status.TODO, date(2030, 2, 1), ["docs"]),
        ("Fix bug", Status.IN_PROGRESS, date(2030, 1, 15), ["bug", "p1"]),
        ("Ship release", Status.TODO, None, ["p1"]),
        ("Review PR", Status.DONE, date(2030, 1, 20), ["review"]),
        ("Plan sprint", Status.TODO, date(2030, 1, 10), ["p1"]),
    ]:
        store.create(TaskCreate(title=title, status=status, due_date=due, tags=tags))
    return store


def create_app() -> FastAPI:
    app = FastAPI(title="Listing demo")
    install_error_handlers(app)
    store = seeded_store()

    @app.get("/tasks")
    def list_tasks(
        status: Status | None = None,
        tag: str | None = None,
        sort: SortKey = "-created_at",
        limit: Annotated[int, Query(ge=1)] = DEFAULT_LIMIT,
        offset: Annotated[int, Query(ge=0)] = 0,
    ) -> Page:
        limit = min(limit, MAX_LIMIT)  # oversized limits are clamped, not rejected
        items, total = store.query(status=status, tag=tag, sort=sort, limit=limit, offset=offset)
        return Page(items=items, total=total, limit=limit, offset=offset)

    return app


if __name__ == "__main__":
    serve(create_app())
