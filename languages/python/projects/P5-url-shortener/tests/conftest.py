import sys
from dataclasses import replace
from pathlib import Path
from typing import Any

import pytest
from fastapi.testclient import TestClient
from p5_helpers import SECRET, Env, FakeClock, MakeEnv

sys.path.insert(0, str(Path(__file__).resolve().parent.parent / "starter"))

from p5_url_shortener.app import create_app
from p5_url_shortener.config import Config


@pytest.fixture
def make_env(tmp_path: Path) -> MakeEnv:
    """Build an app on a temp SQLite file with a fake clock and an in-memory log sink."""

    def build(**overrides: Any) -> Env:
        clock = FakeClock()
        logs: list[str] = []
        db_path = tmp_path / "test.db"
        config = replace(
            Config(
                jwt_secret=SECRET,
                db_path=str(db_path),
                scrypt_n=2**4,  # cheap hashes keep the tests fast
                rate_burst=100,
                clock=clock,
                log_sink=logs.append,
            ),
            **overrides,
        )
        # follow_redirects=False: the redirect itself is the behaviour under test
        client = TestClient(
            create_app(config), follow_redirects=False, raise_server_exceptions=False
        )
        return Env(client, clock, db_path, logs)

    return build


@pytest.fixture
def env(make_env: MakeEnv) -> Env:
    return make_env()
