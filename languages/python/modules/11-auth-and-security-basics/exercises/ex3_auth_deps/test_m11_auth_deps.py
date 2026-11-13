from datetime import timedelta

import jwt
import pytest
from fastapi.testclient import TestClient
from m11_auth_deps import create_app, issue_token

SECRET = "test-secret-test-secret-test-secret-32"


@pytest.fixture
def client() -> TestClient:
    return TestClient(create_app(SECRET))


def bearer(token: str) -> dict[str, str]:
    return {"Authorization": f"Bearer {token}"}


def test_valid_token_reaches_the_handler(client: TestClient) -> None:
    response = client.get("/me", headers=bearer(issue_token(7, "user", SECRET)))

    assert response.status_code == 200
    assert response.json() == {"user_id": 7, "role": "user"}


def test_missing_credentials_get_a_401_envelope_with_a_challenge(client: TestClient) -> None:
    response = client.get("/me")

    assert response.status_code == 401
    assert response.headers["WWW-Authenticate"] == "Bearer"
    assert response.json()["error"]["code"] == "unauthorized"


def test_other_authorization_schemes_are_not_bearer_tokens(client: TestClient) -> None:
    response = client.get("/me", headers={"Authorization": "Basic YW5hOnB3"})

    assert response.status_code == 401


def _forged_tokens() -> dict[str, str]:
    far = 4_102_444_800  # year 2100
    return {
        "wrong-secret": issue_token(7, "admin", "another-secret-another-secret-32!!"),
        "expired": issue_token(7, "admin", SECRET, ttl=timedelta(hours=-1)),
        "alg-none": jwt.encode({"sub": "7", "role": "admin", "exp": far}, None, "none"),
        "other-algorithm": jwt.encode(
            {"sub": "7", "role": "admin", "exp": far}, SECRET * 2, algorithm="HS512"
        ),
        "no-exp": jwt.encode({"sub": "7", "role": "admin"}, SECRET, algorithm="HS256"),
        "no-sub": jwt.encode({"role": "admin", "exp": far}, SECRET, algorithm="HS256"),
        "no-role": jwt.encode({"sub": "7", "exp": far}, SECRET, algorithm="HS256"),
        "sub-not-an-int": jwt.encode(
            {"sub": "seven", "role": "admin", "exp": far}, SECRET, algorithm="HS256"
        ),
        "garbage": "not.a.jwt",
    }


@pytest.mark.parametrize("name", list(_forged_tokens()))
def test_bad_tokens_are_rejected_with_the_same_401(client: TestClient, name: str) -> None:
    response = client.get("/me", headers=bearer(_forged_tokens()[name]))

    assert response.status_code == 401
    assert response.json() == {
        "error": {"code": "unauthorized", "message": "invalid or missing token"}
    }


def test_role_check_returns_403_for_the_wrong_role(client: TestClient) -> None:
    response = client.get("/admin/ping", headers=bearer(issue_token(7, "user", SECRET)))

    assert response.status_code == 403
    assert response.json()["error"]["code"] == "forbidden"
    assert "WWW-Authenticate" not in response.headers


def test_role_check_lets_the_right_role_through(client: TestClient) -> None:
    response = client.get("/admin/ping", headers=bearer(issue_token(1, "admin", SECRET)))

    assert response.status_code == 200
    assert response.json() == {"pong": "admin"}


def test_role_check_without_a_token_is_401_not_403(client: TestClient) -> None:
    assert client.get("/admin/ping").status_code == 401


def test_several_roles_may_be_allowed(client: TestClient) -> None:
    for role, expected in [("admin", 204), ("maintainer", 204), ("user", 403)]:
        response = client.delete("/tasks/1", headers=bearer(issue_token(2, role, SECRET)))
        assert response.status_code == expected, role
