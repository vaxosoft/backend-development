"""Exercise 3: a small, safe migration runner based on PRAGMA user_version."""

import sqlite3
from collections.abc import Sequence
from dataclasses import dataclass


@dataclass(frozen=True, slots=True)
class Migration:
    version: int
    statements: tuple[str, ...]


class MigrationError(Exception):
    pass


def current_version(conn: sqlite3.Connection) -> int:
    return int(conn.execute("PRAGMA user_version").fetchone()[0])


def migrate(conn: sqlite3.Connection, migrations: Sequence[Migration]) -> list[int]:
    """Apply the pending migrations in order and return the versions that were applied.

    `conn` is opened with isolation_level=None (autocommit), so you issue BEGIN/COMMIT yourself.

    - versions must be exactly 1, 2, 3, ... in order, otherwise raise MigrationError
      BEFORE touching the database
    - a database whose user_version is higher than the last migration is newer than this
      code: raise MigrationError
    - each migration runs in its own transaction together with `PRAGMA user_version = N`
      (PRAGMA takes no bound parameters). If a statement fails, roll that migration back
      completely (DDL included), leave earlier migrations applied, and let the sqlite3.Error
      propagate
    - running again applies nothing and returns []
    """
    raise NotImplementedError
