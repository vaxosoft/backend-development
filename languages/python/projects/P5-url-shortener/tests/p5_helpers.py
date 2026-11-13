from collections.abc import Callable
from dataclasses import dataclass
from pathlib import Path

from fastapi.testclient import TestClient

SECRET = "test-secret-test-secret-test-secret-0123"


class FakeClock:
    def __init__(self) -> None:
        self.now = 1_000.0

    def __call__(self) -> float:
        return self.now

    def advance(self, seconds: float) -> None:
        self.now += seconds


@dataclass(slots=True)
class Env:
    client: TestClient
    clock: FakeClock
    db_path: Path
    logs: list[str]


MakeEnv = Callable[..., Env]


PASSWORD = "correct horse battery"


def register(client: TestClient, email: str) -> None:
    response = client.post("/auth/register", json={"email": email, "password": PASSWORD})
    assert response.status_code == 201, response.text


def login(client: TestClient, email: str) -> str:
    response = client.post("/auth/login", json={"email": email, "password": PASSWORD})
    assert response.status_code == 200, response.text
    return str(response.json()["token"])


def signup(client: TestClient, email: str) -> dict[str, str]:
    """Register and log in; return the Authorization header for later requests."""
    register(client, email)
    return {"Authorization": f"Bearer {login(client, email)}"}
