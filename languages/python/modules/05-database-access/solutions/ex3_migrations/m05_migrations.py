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
    if [m.version for m in migrations] != list(range(1, len(migrations) + 1)):
        raise MigrationError("versions must be 1, 2, 3, ... in order")
    version = current_version(conn)
    if version > len(migrations):
        raise MigrationError(f"database version {version} is newer than the code")

    applied: list[int] = []
    for migration in migrations[version:]:
        conn.execute("BEGIN IMMEDIATE")
        try:
            for statement in migration.statements:
                conn.execute(statement)
            conn.execute(f"PRAGMA user_version = {migration.version}")
        except BaseException:
            conn.execute("ROLLBACK")
            raise
        conn.execute("COMMIT")
        applied.append(migration.version)
    return applied
