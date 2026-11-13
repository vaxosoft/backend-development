import json
import re
import sqlite3
import time
from contextlib import closing

import httpx
import jwt
import pytest
from p5_helpers import PASSWORD, SECRET, Env, MakeEnv, login, register, signup

NOW = int(time.time())


def make_link(
    env: Env, headers: dict[str, str], url: str = "https://example.com/a", **extra: str
) -> httpx.Response:
    response: httpx.Response = env.client.post(
        "/links", json={"url": url, **extra}, headers=headers
    )
    return response


# --- registration and login -----------------------------------------------------------


def test_register_returns_201_and_never_echoes_the_password(env: Env) -> None:
    response = env.client.post(
        "/auth/register", json={"email": "Ann@Example.com", "password": PASSWORD}
    )

    assert response.status_code == 201
    assert response.json() == {"id": 1, "email": "ann@example.com"}


@pytest.mark.parametrize(
    ("payload", "field"),
    [
        pytest.param({"email": "ann@example.com", "password": "short"}, "password", id="password"),
        pytest.param({"email": "not-an-email", "password": PASSWORD}, "email", id="email"),
    ],
)
def test_register_rejects_invalid_input_with_422(
    env: Env, payload: dict[str, str], field: str
) -> None:
    response = env.client.post("/auth/register", json=payload)

    assert response.status_code == 422
    error = response.json()["error"]
    assert error["code"] == "validation_failed"
    assert field in error["fields"]


def test_register_twice_with_the_same_email_is_409(env: Env) -> None:
    register(env.client, "ann@example.com")

    response = env.client.post(
        "/auth/register", json={"email": "ann@example.com", "password": PASSWORD}
    )

    assert response.status_code == 409
    assert response.json()["error"]["code"] == "conflict"


def test_password_is_stored_as_a_salted_scrypt_hash(env: Env) -> None:
    register(env.client, "ann@example.com")
    register(env.client, "bob@example.com")

    with closing(sqlite3.connect(env.db_path)) as conn:
        hashes = [row[0] for row in conn.execute("SELECT password_hash FROM users")]

    assert all(h.startswith("scrypt$") and PASSWORD not in h for h in hashes)
    assert hashes[0] != hashes[1]  # same password, different salt


def test_login_returns_an_hs256_jwt_valid_for_one_hour(env: Env) -> None:
    register(env.client, "ann@example.com")

    token = login(env.client, "ann@example.com")

    assert jwt.get_unverified_header(token)["alg"] == "HS256"
    claims = jwt.decode(token, SECRET, algorithms=["HS256"])
    assert claims["sub"] == "1"
    assert claims["exp"] - claims["iat"] == 3600


def test_unknown_user_and_wrong_password_get_the_same_401(env: Env) -> None:
    register(env.client, "ann@example.com")

    wrong_password = env.client.post(
        "/auth/login", json={"email": "ann@example.com", "password": "wrong password"}
    )
    unknown_user = env.client.post(
        "/auth/login", json={"email": "nobody@example.com", "password": PASSWORD}
    )

    assert wrong_password.status_code == unknown_user.status_code == 401
    assert wrong_password.json() == unknown_user.json()
    assert wrong_password.json()["error"]["code"] == "unauthorized"


@pytest.mark.parametrize(
    "headers",
    [
        pytest.param({}, id="no-header"),
        pytest.param({"Authorization": "Bearer not-a-jwt"}, id="garbage"),
        pytest.param(
            {"Authorization": "Bearer " + jwt.encode({"sub": "1", "exp": NOW - 10}, SECRET)},
            id="expired",
        ),
        pytest.param(
            {"Authorization": "Bearer " + jwt.encode({"sub": "1", "exp": NOW + 600}, "x" * 40)},
            id="wrong-secret",
        ),
        pytest.param(
            {"Authorization": "Bearer " + jwt.encode({"sub": "1", "exp": NOW + 600}, "", "none")},
            id="alg-none",
        ),
    ],
)
def test_protected_routes_reject_missing_or_bad_tokens_with_401(
    env: Env, headers: dict[str, str]
) -> None:
    for response in (
        env.client.get("/links", headers=headers),
        env.client.post("/links", json={"url": "https://example.com"}, headers=headers),
        env.client.delete("/links/anything", headers=headers),
    ):
        assert response.status_code == 401
        assert response.json()["error"]["code"] == "unauthorized"


# --- creating links -------------------------------------------------------------------


def test_create_link_with_a_chosen_slug_returns_201(env: Env) -> None:
    headers = signup(env.client, "ann@example.com")

    response = make_link(env, headers, "https://example.com/docs?page=2", slug="my-docs_1")

    assert response.status_code == 201
    assert response.json() == {"slug": "my-docs_1", "url": "https://example.com/docs?page=2"}
    assert response.headers["location"] == "/my-docs_1"


def test_generated_slug_has_seven_alphanumeric_characters(env: Env) -> None:
    headers = signup(env.client, "ann@example.com")

    slugs = {make_link(env, headers).json()["slug"] for _ in range(5)}

    assert len(slugs) == 5
    assert all(re.fullmatch(r"[A-Za-z0-9]{7}", slug) for slug in slugs)


@pytest.mark.parametrize(
    "url",
    [
        "ftp://example.com/file",
        "javascript:alert(1)",
        "example.com/no-scheme",
        "/relative",
        "http://",
    ],
)
def test_url_must_be_an_absolute_http_or_https_url(env: Env, url: str) -> None:
    headers = signup(env.client, "ann@example.com")

    response = make_link(env, headers, url)

    assert response.status_code == 422
    assert "url" in response.json()["error"]["fields"]


@pytest.mark.parametrize("slug", ["ab", "a" * 33, "has space", "slash/inside", "dot.dot"])
def test_slug_must_be_3_to_32_url_safe_characters(env: Env, slug: str) -> None:
    headers = signup(env.client, "ann@example.com")

    response = make_link(env, headers, slug=slug)

    assert response.status_code == 422
    assert "slug" in response.json()["error"]["fields"]


def test_duplicate_slug_is_409_even_for_another_user(env: Env) -> None:
    ann = signup(env.client, "ann@example.com")
    bob = signup(env.client, "bob@example.com")
    make_link(env, ann, slug="taken")

    response = make_link(env, bob, "https://example.org", slug="taken")

    assert response.status_code == 409
    assert response.json()["error"]["code"] == "conflict"


# --- following, listing, deleting -----------------------------------------------------


def test_short_link_redirects_with_302_without_authentication(env: Env) -> None:
    headers = signup(env.client, "ann@example.com")
    make_link(env, headers, "https://example.com/target", slug="land")

    response = env.client.get("/land")

    assert response.status_code == 302
    assert response.headers["location"] == "https://example.com/target"


def test_unknown_slug_is_404_with_the_error_envelope(env: Env) -> None:
    response = env.client.get("/nope")

    assert response.status_code == 404
    assert response.json()["error"]["code"] == "not_found"


def test_every_redirect_increments_the_hit_counter(env: Env) -> None:
    headers = signup(env.client, "ann@example.com")
    make_link(env, headers, slug="land")
    for _ in range(3):
        env.client.get("/land")

    items = env.client.get("/links", headers=headers).json()["items"]

    assert [(item["slug"], item["hits"]) for item in items] == [("land", 3)]


def test_list_shows_only_the_callers_links(env: Env) -> None:
    ann = signup(env.client, "ann@example.com")
    bob = signup(env.client, "bob@example.com")
    make_link(env, ann, slug="ann-one")
    make_link(env, ann, slug="ann-two")
    make_link(env, bob, slug="bob-one")

    body = env.client.get("/links", headers=ann).json()

    assert [item["slug"] for item in body["items"]] == ["ann-two", "ann-one"]
    assert body["total"] == 2
    assert (body["limit"], body["offset"]) == (20, 0)


def test_list_limit_is_clamped_and_bad_values_are_400(env: Env) -> None:
    headers = signup(env.client, "ann@example.com")

    clamped = env.client.get("/links?limit=1000", headers=headers)
    invalid = env.client.get("/links?limit=many", headers=headers)

    assert clamped.json()["limit"] == 100
    assert invalid.status_code == 400
    assert invalid.json()["error"]["code"] == "bad_request"


def test_owner_can_delete_a_link_and_it_stops_redirecting(env: Env) -> None:
    headers = signup(env.client, "ann@example.com")
    make_link(env, headers, slug="land")

    deleted = env.client.delete("/links/land", headers=headers)

    assert deleted.status_code == 204
    assert env.client.get("/land").status_code == 404


def test_deleting_someone_elses_link_is_403_and_keeps_the_link(env: Env) -> None:
    ann = signup(env.client, "ann@example.com")
    bob = signup(env.client, "bob@example.com")
    make_link(env, ann, slug="land")

    response = env.client.delete("/links/land", headers=bob)

    assert response.status_code == 403
    assert response.json()["error"]["code"] == "forbidden"
    assert env.client.get("/land").status_code == 302


# --- rate limiting --------------------------------------------------------------------


def test_creating_links_beyond_the_burst_is_429_with_retry_after(make_env: MakeEnv) -> None:
    env = make_env(rate_burst=2, rate_refill_per_second=0.5)
    headers = signup(env.client, "ann@example.com")
    assert make_link(env, headers).status_code == 201
    assert make_link(env, headers).status_code == 201

    response = make_link(env, headers)

    assert response.status_code == 429
    assert response.headers["retry-after"] == "2"
    assert response.json()["error"]["code"] == "rate_limited"


def test_tokens_refill_as_time_passes(make_env: MakeEnv) -> None:
    env = make_env(rate_burst=1, rate_refill_per_second=0.5)
    headers = signup(env.client, "ann@example.com")
    make_link(env, headers)
    assert make_link(env, headers).status_code == 429

    env.clock.advance(2)

    assert make_link(env, headers).status_code == 201


def test_each_user_has_their_own_bucket(make_env: MakeEnv) -> None:
    env = make_env(rate_burst=1, rate_refill_per_second=0.01)
    ann = signup(env.client, "ann@example.com")
    bob = signup(env.client, "bob@example.com")
    make_link(env, ann)
    assert make_link(env, ann).status_code == 429

    assert make_link(env, bob).status_code == 201


# --- structured logging ---------------------------------------------------------------


def test_every_request_logs_one_json_line_with_the_standard_fields(env: Env) -> None:
    env.client.get("/healthz-does-not-exist")
    env.client.get("/also-missing?secret=1")

    records = [json.loads(line) for line in env.logs]

    assert len(records) == 2
    assert records[0].keys() == {"request_id", "method", "path", "status", "duration_ms"}
    assert (records[0]["method"], records[0]["path"], records[0]["status"]) == (
        "GET",
        "/healthz-does-not-exist",
        404,
    )
    assert records[1]["path"] == "/also-missing"  # the query string is never logged
    assert isinstance(records[0]["duration_ms"], int | float)


def test_user_id_is_logged_only_when_the_caller_is_known(env: Env) -> None:
    headers = signup(env.client, "ann@example.com")
    env.logs.clear()

    make_link(env, headers)
    env.client.get("/missing")
    with_user, without_user = (json.loads(line) for line in env.logs)

    assert with_user["user_id"] == 1
    assert "user_id" not in without_user


def test_passwords_and_tokens_never_reach_the_log(env: Env) -> None:
    register(env.client, "ann@example.com")
    token = login(env.client, "ann@example.com")
    make_link(env, {"Authorization": f"Bearer {token}"})

    everything = "\n".join(env.logs)

    assert len(env.logs) == 3
    assert PASSWORD not in everything
    assert token not in everything
    assert "Bearer" not in everything


def test_request_id_is_echoed_when_safe_and_generated_otherwise(env: Env) -> None:
    echoed = env.client.get("/a", headers={"X-Request-ID": "trace-123"})
    replaced = env.client.get("/b", headers={"X-Request-ID": "not safe, has spaces"})
    generated = env.client.get("/c")

    assert echoed.headers["x-request-id"] == "trace-123"
    assert replaced.headers["x-request-id"] != "not safe, has spaces"
    assert generated.headers["x-request-id"]
    logged_ids = [json.loads(line)["request_id"] for line in env.logs]
    assert logged_ids == [r.headers["x-request-id"] for r in (echoed, replaced, generated)]
