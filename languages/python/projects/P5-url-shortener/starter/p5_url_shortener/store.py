"""Persistence for users and links (SQLite through the standard library)."""

import sqlite3
import threading

from .models import Link, User


class EmailTakenError(Exception):
    pass


class SlugTakenError(Exception):
    pass


class Store:
    def __init__(self, conn: sqlite3.Connection) -> None:
        self._conn = conn
        self._lock = threading.Lock()

    def create_user(self, email: str, password_hash: str) -> User:
        """Insert a user; raise EmailTakenError when the email exists."""
        raise NotImplementedError("TODO: INSERT INTO users with bound parameters")

    def get_user_by_email(self, email: str) -> User | None:
        raise NotImplementedError("TODO: SELECT the user or return None")

    def create_link(self, owner_id: int, slug: str, url: str, created_at: str) -> Link:
        """Insert a link; raise SlugTakenError when the slug exists."""
        raise NotImplementedError("TODO: INSERT INTO links")

    def get_link(self, slug: str) -> Link | None:
        raise NotImplementedError("TODO: SELECT the link or return None")

    def record_hit(self, slug: str) -> str | None:
        """Atomically add one hit and return the target URL (None when unknown)."""
        raise NotImplementedError("TODO: UPDATE links SET hits = hits + 1 ... RETURNING url")

    def list_links(self, owner_id: int, limit: int, offset: int) -> tuple[list[Link], int]:
        """Return one page of the owner's links (newest first) and the total count."""
        raise NotImplementedError("TODO: SELECT with LIMIT/OFFSET plus COUNT(*)")

    def delete_link(self, slug: str) -> None:
        raise NotImplementedError("TODO: DELETE FROM links")
