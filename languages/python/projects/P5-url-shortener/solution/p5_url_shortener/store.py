"""Persistence for users and links (SQLite through the standard library)."""

import sqlite3
import threading

from .models import Link, User


class EmailTakenError(Exception):
    pass


class SlugTakenError(Exception):
    pass


def _link(row: sqlite3.Row) -> Link:
    return Link(row["slug"], row["url"], row["owner_id"], row["hits"], row["created_at"])


class Store:
    def __init__(self, conn: sqlite3.Connection) -> None:
        self._conn = conn
        self._lock = threading.Lock()

    def create_user(self, email: str, password_hash: str) -> User:
        """Insert a user; raise EmailTakenError when the email exists."""
        try:
            with self._lock, self._conn:
                cursor = self._conn.execute(
                    "INSERT INTO users (email, password_hash) VALUES (?, ?)",
                    (email, password_hash),
                )
        except sqlite3.IntegrityError as err:
            raise EmailTakenError(email) from err
        assert cursor.lastrowid is not None
        return User(cursor.lastrowid, email, password_hash)

    def get_user_by_email(self, email: str) -> User | None:
        with self._lock:
            row = self._conn.execute(
                "SELECT id, email, password_hash FROM users WHERE email = ?", (email,)
            ).fetchone()
        return User(row["id"], row["email"], row["password_hash"]) if row else None

    def create_link(self, owner_id: int, slug: str, url: str, created_at: str) -> Link:
        """Insert a link; raise SlugTakenError when the slug exists."""
        try:
            with self._lock, self._conn:
                self._conn.execute(
                    "INSERT INTO links (slug, url, owner_id, created_at) VALUES (?, ?, ?, ?)",
                    (slug, url, owner_id, created_at),
                )
        except sqlite3.IntegrityError as err:
            raise SlugTakenError(slug) from err
        return Link(slug, url, owner_id, 0, created_at)

    def get_link(self, slug: str) -> Link | None:
        with self._lock:
            row = self._conn.execute("SELECT * FROM links WHERE slug = ?", (slug,)).fetchone()
        return _link(row) if row else None

    def record_hit(self, slug: str) -> str | None:
        """Atomically add one hit and return the target URL (None when unknown)."""
        with self._lock, self._conn:
            row = self._conn.execute(
                "UPDATE links SET hits = hits + 1 WHERE slug = ? RETURNING url", (slug,)
            ).fetchone()
        return str(row["url"]) if row else None

    def list_links(self, owner_id: int, limit: int, offset: int) -> tuple[list[Link], int]:
        """Return one page of the owner's links (newest first) and the total count."""
        with self._lock:
            rows = self._conn.execute(
                "SELECT * FROM links WHERE owner_id = ? "
                "ORDER BY created_at DESC, rowid DESC LIMIT ? OFFSET ?",
                (owner_id, limit, offset),
            ).fetchall()
            total = self._conn.execute(
                "SELECT COUNT(*) FROM links WHERE owner_id = ?", (owner_id,)
            ).fetchone()[0]
        return [_link(row) for row in rows], int(total)

    def delete_link(self, slug: str) -> None:
        with self._lock, self._conn:
            self._conn.execute("DELETE FROM links WHERE slug = ?", (slug,))
