import sqlite3
from collections.abc import Iterator
from contextlib import closing

import pytest
from m05_migrations import Migration, MigrationError, current_version, migrate

V1 = Migration(1, ("CREATE TABLE tasks (id INTEGER PRIMARY KEY, title TEXT NOT NULL)",))
V2 = Migration(
    2,
    (
        "ALTER TABLE tasks ADD COLUMN due_date TEXT",
        "CREATE TABLE task_tags (task_id INTEGER NOT NULL, tag TEXT NOT NULL)",
    ),
)


@pytest.fixture
def conn() -> Iterator[sqlite3.Connection]:
    with closing(sqlite3.connect(":memory:", isolation_level=None)) as c:
        yield c


def tables(conn: sqlite3.Connection) -> list[str]:
    rows = conn.execute("SELECT name FROM sqlite_master WHERE type = 'table' ORDER BY name")
    return [r[0] for r in rows]


def test_applies_all_pending_migrations_in_order(conn: sqlite3.Connection) -> None:
    assert migrate(conn, [V1, V2]) == [1, 2]
    assert current_version(conn) == 2
    assert tables(conn) == ["task_tags", "tasks"]


def test_second_run_is_a_no_op(conn: sqlite3.Connection) -> None:
    migrate(conn, [V1, V2])
    assert migrate(conn, [V1, V2]) == []


def test_only_new_migrations_run_after_an_upgrade(conn: sqlite3.Connection) -> None:
    assert migrate(conn, [V1]) == [1]
    assert migrate(conn, [V1, V2]) == [2]
    assert current_version(conn) == 2


def test_failing_migration_is_rolled_back_completely(conn: sqlite3.Connection) -> None:
    broken = Migration(
        2,
        ("CREATE TABLE task_tags (task_id INTEGER)", "THIS IS NOT SQL"),
    )
    with pytest.raises(sqlite3.Error):
        migrate(conn, [V1, broken])
    assert current_version(conn) == 1
    assert tables(conn) == ["tasks"]
    assert not conn.in_transaction


def test_a_fixed_migration_can_be_applied_after_a_failure(conn: sqlite3.Connection) -> None:
    broken = Migration(2, ("CREATE TABLE task_tags (task_id INTEGER)", "NOPE"))
    with pytest.raises(sqlite3.Error):
        migrate(conn, [V1, broken])
    assert migrate(conn, [V1, V2]) == [2]
    assert current_version(conn) == 2


def test_database_newer_than_code_is_refused(conn: sqlite3.Connection) -> None:
    migrate(conn, [V1, V2])
    with pytest.raises(MigrationError, match="newer"):
        migrate(conn, [V1])
    assert current_version(conn) == 2


@pytest.mark.parametrize(
    "migrations",
    [
        [Migration(2, ("CREATE TABLE a (x INTEGER)",))],
        [V1, Migration(3, ("CREATE TABLE a (x INTEGER)",))],
        [V1, V1],
    ],
)
def test_invalid_version_sequences_touch_nothing(
    conn: sqlite3.Connection, migrations: list[Migration]
) -> None:
    with pytest.raises(MigrationError):
        migrate(conn, migrations)
    assert current_version(conn) == 0
    assert tables(conn) == []
