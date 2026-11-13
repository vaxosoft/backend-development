"""Test doubles: fake clock and notifier, mocks with and without autospec, patching.

Run: pytest modules/12-testing/examples/03_fakes_and_mocks.py -vv --no-header
"""

from datetime import UTC, datetime, timedelta, tzinfo
from unittest.mock import Mock, create_autospec

import m12_tasks
import pytest
from m12_tasks import InMemoryTaskRepository, NotFoundError, Task, TaskRepository, TaskService

NOON = datetime(2030, 1, 1, 12, 0, tzinfo=UTC)


class FakeClock:
    def __init__(self, now: datetime = NOON) -> None:
        self.now = now

    def __call__(self) -> datetime:
        return self.now

    def advance(self, delta: timedelta) -> None:
        self.now += delta


class RecordingNotifier:
    """A hand-written fake: simple, typed, and it reads like the behaviour it stands for."""

    def __init__(self) -> None:
        self.created: list[Task] = []

    def task_created(self, task: Task) -> None:
        self.created.append(task)


def test_fake_clock_makes_time_deterministic() -> None:
    clock = FakeClock()
    service = TaskService(InMemoryTaskRepository(), clock=clock)

    first = service.create("first")
    clock.advance(timedelta(hours=1))
    second = service.create("second")

    assert first.created_at == NOON
    assert second.created_at == NOON + timedelta(hours=1)


def test_fake_notifier_records_what_happened() -> None:
    notifier = RecordingNotifier()
    service = TaskService(InMemoryTaskRepository(), clock=FakeClock(), notifier=notifier)

    service.create("Write docs", tags=["Docs"])

    assert [(t.title, t.tags) for t in notifier.created] == [("Write docs", ("docs",))]


def test_invalid_input_never_reaches_the_notifier() -> None:
    notifier = RecordingNotifier()
    service = TaskService(InMemoryTaskRepository(), notifier=notifier)

    with pytest.raises(ValueError, match="title"):
        service.create("   ")

    assert notifier.created == []


def test_mock_verifies_the_interaction_with_the_repository() -> None:
    repo = create_autospec(TaskRepository, instance=True)
    repo.add.return_value = Task(1, "Write docs", "todo", (), NOON)
    service = TaskService(repo, clock=FakeClock())

    service.create("  Write docs ")

    repo.add.assert_called_once_with("Write docs", (), NOON)


def test_mock_can_simulate_a_missing_row() -> None:
    repo = create_autospec(TaskRepository, instance=True)
    repo.get.return_value = None

    with pytest.raises(NotFoundError):
        TaskService(repo).get(99)


def test_plain_mock_accepts_calls_the_real_object_would_reject() -> None:
    sloppy = Mock()
    sloppy.add("title")  # wrong signature (3 parameters required): silently accepted
    sloppy.no_such_method()  # does not exist on TaskRepository: silently accepted

    strict = create_autospec(TaskRepository, instance=True)
    with pytest.raises(TypeError):
        strict.add("title")
    with pytest.raises(AttributeError):
        strict.no_such_method()


def test_patch_the_name_where_it_is_looked_up(monkeypatch: pytest.MonkeyPatch) -> None:
    class FrozenDatetime(datetime):
        @classmethod
        def now(cls, tz: tzinfo | None = None) -> FrozenDatetime:
            return cls(2031, 5, 6, 7, 8, tzinfo=UTC)

    # The default clock calls `datetime.now` via m12_tasks's namespace, so that is the name to
    # patch: patching `datetime.datetime` globally would not touch the already-imported name.
    # An injected FakeClock (above) is simpler; patching is the fallback for code you cannot change.
    monkeypatch.setattr(m12_tasks, "datetime", FrozenDatetime)
    service = TaskService(InMemoryTaskRepository())

    assert service.create("patched").created_at == datetime(2031, 5, 6, 7, 8, tzinfo=UTC)


if __name__ == "__main__":
    raise SystemExit(pytest.main([__file__, "-vv", "--no-header"]))
