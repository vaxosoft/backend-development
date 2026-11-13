"""Fixtures (setup/teardown, scope, factories), tmp_path, monkeypatch, capsys and caplog.

Run: pytest modules/12-testing/examples/02_fixtures.py -vv --no-header
"""

import json
import logging
import os
from collections.abc import Callable, Iterator
from datetime import UTC, datetime
from pathlib import Path

import pytest
from m12_tasks import SqliteTaskRepository, Task

log = logging.getLogger("tracker.export")


def page_size() -> int:
    return int(os.environ.get("PAGE_SIZE", "20"))


def export_tasks(tasks: list[Task], path: Path) -> None:
    payload = [{"id": t.id, "title": t.title, "status": t.status} for t in tasks]
    path.write_text(json.dumps(payload), encoding="utf-8")
    log.info("exported %d tasks", len(tasks))


@pytest.fixture
def repo() -> Iterator[SqliteTaskRepository]:
    repo = SqliteTaskRepository()  # setup: runs before each test that asks for `repo`
    yield repo
    repo.close()  # teardown: runs after the test, even if it failed


@pytest.fixture
def make_task(repo: SqliteTaskRepository) -> Callable[[str], Task]:
    """A factory fixture: the test decides how many tasks and what they look like."""
    moment = datetime(2030, 1, 1, 9, 0, tzinfo=UTC)
    return lambda title: repo.add(title, (), moment)


@pytest.fixture(scope="module")
def expensive_constant() -> int:
    return 42  # module scope: built once for all tests in this file


def test_fixtures_compose(repo: SqliteTaskRepository, make_task: Callable[[str], Task]) -> None:
    make_task("one")
    make_task("two")

    assert [t.title for t in repo.list_all()] == ["one", "two"]


def test_each_test_gets_a_fresh_repo(repo: SqliteTaskRepository) -> None:
    assert repo.list_all() == []  # nothing leaked from the previous test


def test_module_scoped_fixture(expensive_constant: int) -> None:
    assert expensive_constant == 42


def test_tmp_path_is_a_private_directory(tmp_path: Path, make_task: Callable[[str], Task]) -> None:
    target = tmp_path / "tasks.json"

    export_tasks([make_task("Write docs")], target)

    assert json.loads(target.read_text(encoding="utf-8")) == [
        {"id": 1, "title": "Write docs", "status": "todo"}
    ]


def test_monkeypatch_env_is_undone_after_the_test(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("PAGE_SIZE", "5")
    assert page_size() == 5


def test_default_page_size(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("PAGE_SIZE", raising=False)  # whatever the developer's shell has
    assert page_size() == 20


def test_caplog_captures_log_records(tmp_path: Path, caplog: pytest.LogCaptureFixture) -> None:
    caplog.set_level(logging.INFO, logger="tracker.export")

    export_tasks([], tmp_path / "empty.json")

    assert [r.getMessage() for r in caplog.records] == ["exported 0 tasks"]


def test_capsys_captures_printed_output(capsys: pytest.CaptureFixture[str]) -> None:
    print("hello")

    assert capsys.readouterr().out == "hello\n"


if __name__ == "__main__":
    raise SystemExit(pytest.main([__file__, "-vv", "--no-header"]))
