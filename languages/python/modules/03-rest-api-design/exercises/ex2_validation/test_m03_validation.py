from typing import Any

import pytest
from fastapi.testclient import TestClient
from m03_validation import create_app


@pytest.fixture
def client() -> TestClient:
    return TestClient(create_app(), raise_server_exceptions=False)


def post(client: TestClient, body: dict[str, Any]) -> Any:
    return client.post("/tasks", json=body)


def test_valid_task_is_created_and_normalised(client: TestClient) -> None:
    response = post(client, {"title": "  Write docs  ", "due_date": "2030-01-31", "tags": ["a"]})
    assert response.status_code == 201
    assert response.json() == {
        "id": 1,
        "title": "Write docs",
        "status": "todo",
        "due_date": "2030-01-31",
        "tags": ["a"],
    }


@pytest.mark.parametrize(
    ("body", "bad_field"),
    [
        ({}, "title"),
        ({"title": ""}, "title"),
        ({"title": "   "}, "title"),
        ({"title": "x" * 201}, "title"),
        ({"title": "a", "status": "finished"}, "status"),
        ({"title": "a", "due_date": "31-01-2030"}, "due_date"),
        ({"title": "a", "tags": ["Upper"]}, "tags"),
        ({"title": "a", "tags": ["dup", "dup"]}, "tags"),
        ({"title": "a", "tags": [f"t{i}" for i in range(11)]}, "tags"),
        ({"title": "a", "tags": [""]}, "tags.0"),
        ({"title": "a", "tags": ["x" * 31]}, "tags.0"),
        ({"title": "a", "colour": "red"}, "colour"),
    ],
)
def test_invalid_body_is_422_with_field(
    client: TestClient, body: dict[str, Any], bad_field: str
) -> None:
    response = post(client, body)
    assert response.status_code == 422
    error = response.json()["error"]
    assert error["code"] == "validation_failed"
    assert bad_field in error["fields"]
    assert all(isinstance(message, str) and message for message in error["fields"].values())


def test_all_invalid_fields_are_reported_together(client: TestClient) -> None:
    fields = post(client, {"title": "", "status": "x"}).json()["error"]["fields"]
    assert {"title", "status"} <= set(fields)


def test_malformed_json_is_400(client: TestClient) -> None:
    response = client.post(
        "/tasks", content='{"title": ', headers={"Content-Type": "application/json"}
    )
    assert response.status_code == 400
    assert response.json()["error"]["code"] == "bad_request"
    assert "fields" not in response.json()["error"]


@pytest.mark.parametrize("query", ["text=a&count=0", "text=a&count=6", "text=a&count=x", "count=2"])
def test_bad_query_values_are_400_not_422(client: TestClient, query: str) -> None:
    response = client.get(f"/repeat?{query}")
    assert response.status_code == 400
    assert response.json()["error"]["code"] == "bad_request"
    assert response.json()["error"]["fields"]


def test_good_query_still_works(client: TestClient) -> None:
    assert client.get("/repeat?text=ab&count=2").json() == {"result": "abab"}


def test_unknown_route_uses_the_envelope(client: TestClient) -> None:
    response = client.get("/nope")
    assert response.status_code == 404
    assert response.json()["error"]["code"] == "not_found"


def test_wrong_method_uses_the_envelope(client: TestClient) -> None:
    response = client.delete("/tasks")
    assert response.status_code == 405
    assert response.json()["error"]["code"] == "method_not_allowed"


def test_unexpected_exception_is_500_without_details(client: TestClient) -> None:
    response = client.get("/boom")
    assert response.status_code == 500
    assert response.json() == {"error": {"code": "internal", "message": "internal server error"}}
    assert "secret" not in response.text
