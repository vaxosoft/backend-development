"""Resource modelling, HTTP verbs and status codes on a tiny in-memory "tasks" resource.

Run:  python 01_resources.py   (ADDR=host:port to change the address)
Errors still use FastAPI's default {"detail": ...} body here; example 02 replaces it.
"""

from itertools import count
from typing import Annotated

from fastapi import FastAPI, HTTPException, Response
from m03_errors import serve
from pydantic import BaseModel, Field


class TaskIn(BaseModel):
    title: Annotated[str, Field(min_length=1, max_length=200)]
    done: bool = False


class Task(BaseModel):
    id: int
    title: str
    done: bool


class TaskPatch(BaseModel):
    title: Annotated[str, Field(min_length=1, max_length=200)] | None = None
    done: bool | None = None


tasks: dict[int, Task] = {}  # module-level state is fine for a demo; see example 05 for a factory
ids = count(1)

app = FastAPI(title="Resources demo")


def find(task_id: int) -> Task:
    task = tasks.get(task_id)
    if task is None:
        raise HTTPException(status_code=404, detail=f"task {task_id} not found")
    return task


# Collection: nouns in the URL, the verb is the HTTP method.
@app.post("/tasks", status_code=201)
def create(data: TaskIn, response: Response) -> Task:
    task = Task(id=next(ids), **data.model_dump())
    tasks[task.id] = task
    response.headers["Location"] = f"/tasks/{task.id}"
    return task


@app.get("/tasks")
def list_tasks() -> list[Task]:
    return list(tasks.values())


# Item: GET reads (safe), PUT replaces (idempotent), PATCH changes parts, DELETE removes.
@app.get("/tasks/{task_id}")
def read(task_id: int) -> Task:
    return find(task_id)


@app.put("/tasks/{task_id}")
def replace(task_id: int, data: TaskIn) -> Task:
    find(task_id)
    tasks[task_id] = Task(id=task_id, **data.model_dump())
    return tasks[task_id]


@app.patch("/tasks/{task_id}")
def update(task_id: int, patch: TaskPatch) -> Task:
    tasks[task_id] = find(task_id).model_copy(update=patch.model_dump(exclude_unset=True))
    return tasks[task_id]


@app.delete("/tasks/{task_id}", status_code=204)
def delete(task_id: int) -> None:
    find(task_id)
    del tasks[task_id]


if __name__ == "__main__":
    serve(app)
