"""Dataclasses, Enum, Protocol and composition instead of inheritance."""

from dataclasses import dataclass, field, replace
from enum import StrEnum
from typing import Protocol


class Status(StrEnum):
    TODO = "todo"
    IN_PROGRESS = "in_progress"
    DONE = "done"


@dataclass(slots=True, frozen=True)
class Task:
    id: int
    title: str
    status: Status = Status.TODO
    tags: tuple[str, ...] = ()

    def with_status(self, status: Status) -> Task:
        return replace(self, status=status)


@dataclass(slots=True)
class Counter:
    """Mutable dataclass: field(default_factory=...) avoids the shared-default trap."""

    seen: list[str] = field(default_factory=list)

    def add(self, item: str) -> None:
        self.seen.append(item)


class Notifier(Protocol):
    """Structural interface: anything with a matching `notify` method fits."""

    def notify(self, message: str) -> None: ...


class ConsoleNotifier:
    def notify(self, message: str) -> None:
        print("notify:", message)


class RecordingNotifier:
    def __init__(self) -> None:
        self.messages: list[str] = []

    def notify(self, message: str) -> None:
        self.messages.append(message)


class TaskService:
    """Composition: the service has a notifier, it is not one."""

    def __init__(self, notifier: Notifier) -> None:
        self._notifier = notifier

    def complete(self, task: Task) -> Task:
        done = task.with_status(Status.DONE)
        self._notifier.notify(f"task {done.id} is {done.status}")
        return done


def main() -> None:
    task = Task(1, "Write docs", tags=("docs", "p1"))
    print(task)
    print("equal copy:", task == Task(1, "Write docs", tags=("docs", "p1")))

    try:
        task.title = "Other"  # type: ignore[misc]
    except AttributeError as err:
        print("frozen:", type(err).__name__)

    print("status from wire value:", Status("in_progress").name)
    try:
        Status("finished")
    except ValueError as err:
        print("bad status:", err)

    service = TaskService(ConsoleNotifier())
    done = service.complete(task)
    print("original untouched:", task.status, "| new:", done.status)

    recorder = RecordingNotifier()
    TaskService(recorder).complete(task)
    print("recorded:", recorder.messages)

    counter = Counter()
    counter.add("x")
    print("counter:", counter, "| fresh:", Counter())


if __name__ == "__main__":
    main()
