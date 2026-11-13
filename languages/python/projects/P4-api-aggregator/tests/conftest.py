import sys
from collections.abc import Iterator
from dataclasses import replace
from pathlib import Path
from typing import Any

import pytest
from fastapi.testclient import TestClient
from p4_fake_upstream import FakeUpstream, MakeClient, serve

sys.path.insert(0, str(Path(__file__).resolve().parent.parent / "starter"))

from p4_api_aggregator.app import create_app
from p4_api_aggregator.config import Config


@pytest.fixture(scope="module")
def _server() -> Iterator[tuple[FakeUpstream, str]]:
    fake = FakeUpstream()
    with serve(fake.app) as url:
        yield fake, url


@pytest.fixture
def upstream(_server: tuple[FakeUpstream, str]) -> tuple[FakeUpstream, str]:
    """The fake upstream with all counters and behaviours reset."""
    _server[0].reset()
    return _server


@pytest.fixture
def make_client(upstream: tuple[FakeUpstream, str]) -> Iterator[MakeClient]:
    """Build a client for the aggregator; keyword arguments override Config fields."""
    opened: list[TestClient] = []

    def build(**overrides: Any) -> TestClient:
        config = replace(Config(upstream_url=upstream[1], backoff_base=0.001), **overrides)
        # raise_server_exceptions=False: a crashing handler must show up as a 500, not an exception
        client = TestClient(create_app(config), raise_server_exceptions=False)
        client.__enter__()
        opened.append(client)
        return client

    yield build
    for client in opened:
        client.__exit__(None, None, None)
