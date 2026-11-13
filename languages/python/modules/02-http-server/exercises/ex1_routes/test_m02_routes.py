import pytest
from fastapi.testclient import TestClient
from m02_routes import create_app


@pytest.fixture
def client() -> TestClient:
    return TestClient(create_app())


def test_hello_default_and_named(client: TestClient) -> None:
    default = client.get("/hello")
    assert default.status_code == 200
    assert default.text == "Hello, world!"
    assert default.headers["content-type"].startswith("text/plain")
    assert client.get("/hello/Ada").text == "Hello, Ada!"


def test_add_converts_query_to_ints(client: TestClient) -> None:
    assert client.get("/add", params={"a": 2, "b": 40}).json() == {"sum": 42}


@pytest.mark.parametrize("params", [{"a": 1}, {"a": "x", "b": 2}, {}])
def test_add_rejects_missing_or_non_integer(client: TestClient, params: dict[str, str]) -> None:
    response = client.get("/add", params=params)
    assert response.status_code == 422
    assert response.json()["detail"][0]["loc"][0] == "query"


def test_path_parameters_are_typed(client: TestClient) -> None:
    assert client.get("/users/3/tasks/7").json() == {"user_id": 3, "task_id": 7}
    bad_type = client.get("/users/abc/tasks/7")
    assert bad_type.status_code == 422
    assert bad_type.json()["detail"][0]["loc"] == ["path", "user_id"]
    assert client.get("/users/3/tasks/0").status_code == 422


def test_search_defaults(client: TestClient) -> None:
    assert client.get("/search").json() == {"tags": [], "limit": 10}


def test_search_repeated_query_parameter(client: TestClient) -> None:
    response = client.get("/search", params=[("tag", "x"), ("tag", "y"), ("limit", "5")])
    assert response.json() == {"tags": ["x", "y"], "limit": 5}


@pytest.mark.parametrize("limit", ["0", "51", "many"])
def test_search_limit_bounds(client: TestClient, limit: str) -> None:
    assert client.get("/search", params={"limit": limit}).status_code == 422


def test_agent_header(client: TestClient) -> None:
    assert client.get("/agent", headers={"User-Agent": "demo/1.0"}).json() == {
        "user_agent": "demo/1.0"
    }
