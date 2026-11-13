import pytest
from fastapi.testclient import TestClient
from m03_resources import create_app


@pytest.fixture
def client() -> TestClient:
    return TestClient(create_app())


def test_post_returns_201_location_and_body(client: TestClient) -> None:
    response = client.post("/notes", json={"text": "first"})
    assert response.status_code == 201
    assert response.headers["location"] == "/notes/1"
    assert response.json() == {"id": 1, "text": "first"}


def test_ids_increase_and_list_keeps_creation_order(client: TestClient) -> None:
    client.post("/notes", json={"text": "a"})
    client.post("/notes", json={"text": "b"})
    response = client.get("/notes")
    assert response.status_code == 200
    assert response.json() == [{"id": 1, "text": "a"}, {"id": 2, "text": "b"}]


def test_get_one(client: TestClient) -> None:
    client.post("/notes", json={"text": "a"})
    assert client.get("/notes/1").json() == {"id": 1, "text": "a"}


def test_unknown_id_is_404_with_envelope(client: TestClient) -> None:
    response = client.get("/notes/42")
    assert response.status_code == 404
    assert response.json() == {"error": {"code": "not_found", "message": "note 42 not found"}}


def test_put_replaces_and_keeps_the_id(client: TestClient) -> None:
    client.post("/notes", json={"text": "old"})
    response = client.put("/notes/1", json={"text": "new"})
    assert response.status_code == 200
    assert response.json() == {"id": 1, "text": "new"}
    assert client.get("/notes/1").json()["text"] == "new"


def test_put_is_idempotent(client: TestClient) -> None:
    client.post("/notes", json={"text": "old"})
    first = client.put("/notes/1", json={"text": "same"})
    second = client.put("/notes/1", json={"text": "same"})
    assert first.json() == second.json()
    assert len(client.get("/notes").json()) == 1


def test_put_unknown_is_404_and_does_not_create(client: TestClient) -> None:
    assert client.put("/notes/5", json={"text": "x"}).status_code == 404
    assert client.get("/notes").json() == []


def test_delete_returns_204_without_body_then_404(client: TestClient) -> None:
    client.post("/notes", json={"text": "bye"})
    first = client.delete("/notes/1")
    assert first.status_code == 204
    assert first.content == b""
    assert client.delete("/notes/1").status_code == 404
    assert client.get("/notes/1").status_code == 404


def test_ids_are_not_reused_after_delete(client: TestClient) -> None:
    client.post("/notes", json={"text": "a"})
    client.delete("/notes/1")
    assert client.post("/notes", json={"text": "b"}).json()["id"] == 2
