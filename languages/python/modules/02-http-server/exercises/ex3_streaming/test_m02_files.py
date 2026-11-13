import hashlib
from pathlib import Path

import pytest
from fastapi.testclient import TestClient
from m02_files import CHUNK_SIZE, create_app, sanitize_filename


@pytest.fixture
def storage(tmp_path: Path) -> Path:
    return tmp_path / "store"


@pytest.fixture
def client(storage: Path) -> TestClient:
    return TestClient(create_app(storage, max_bytes=1_000_000))


@pytest.mark.parametrize(
    ("raw", "expected"),
    [
        ("report.pdf", "report.pdf"),
        ("../../etc/passwd", "passwd"),
        ("C:\\Users\\x\\evil.exe", "evil.exe"),
        ("..", "upload.bin"),
        (".hidden", "hidden"),
        ('a"b;c.txt', "a_b_c.txt"),
        ("tab\tname.txt", "tab_name.txt"),
        ("", "upload.bin"),
        (None, "upload.bin"),
        ("my file-1.tar.gz", "my file-1.tar.gz"),
    ],
)
def test_sanitize_filename(raw: str | None, expected: str) -> None:
    assert sanitize_filename(raw) == expected


def test_upload_returns_metadata_and_location(client: TestClient) -> None:
    data = b"hello upload"
    response = client.post("/files", files={"file": ("notes.txt", data, "text/plain")})
    digest = hashlib.sha256(data).hexdigest()
    assert response.status_code == 201
    assert response.headers["location"] == f"/files/{digest}"
    assert response.json() == {
        "id": digest,
        "name": "notes.txt",
        "size": len(data),
        "sha256": digest,
        "content_type": "text/plain",
    }


def test_multi_chunk_round_trip_with_download_headers(client: TestClient) -> None:
    data = bytes(range(256)) * ((CHUNK_SIZE * 3) // 256 + 5)
    uploaded = client.post("/files", files={"file": ("big.bin", data, "application/x-demo")})
    file_id = uploaded.json()["id"]

    response = client.get(f"/files/{file_id}")
    assert response.status_code == 200
    assert response.content == data
    assert response.headers["content-type"] == "application/x-demo"
    assert response.headers["content-length"] == str(len(data))
    assert response.headers["etag"] == f'"{file_id}"'
    assert response.headers["content-disposition"] == 'attachment; filename="big.bin"'


def test_conditional_get_returns_304_without_body(client: TestClient) -> None:
    file_id = client.post("/files", files={"file": ("a.txt", b"abc", "text/plain")}).json()["id"]
    response = client.get(f"/files/{file_id}", headers={"If-None-Match": f'"{file_id}"'})
    assert response.status_code == 304
    assert response.content == b""
    stale = client.get(f"/files/{file_id}", headers={"If-None-Match": '"other"'})
    assert stale.status_code == 200


def test_unknown_id_is_404_envelope(client: TestClient) -> None:
    response = client.get("/files/deadbeef")
    assert response.status_code == 404
    assert response.json()["error"]["code"] == "not_found"


def test_too_large_upload_is_413_and_leaves_nothing_behind(storage: Path) -> None:
    client = TestClient(create_app(storage, max_bytes=1000))
    response = client.post("/files", files={"file": ("big.bin", b"x" * 5000, "text/plain")})
    assert response.status_code == 413
    assert response.json()["error"]["code"] == "payload_too_large"
    assert list(storage.iterdir()) == []


def test_upload_of_exactly_the_limit_is_accepted(storage: Path) -> None:
    client = TestClient(create_app(storage, max_bytes=1000))
    response = client.post("/files", files={"file": ("ok.bin", b"x" * 1000, "text/plain")})
    assert response.status_code == 201
    assert [p.name for p in storage.iterdir()] == [response.json()["id"]]


def test_hostile_filename_never_leaves_the_storage_directory(
    tmp_path: Path, storage: Path, client: TestClient
) -> None:
    response = client.post("/files", files={"file": ("../../evil.txt", b"data", "text/plain")})
    assert response.json()["name"] == "evil.txt"
    assert (
        'filename="evil.txt"'
        in client.get(f"/files/{response.json()['id']}").headers["content-disposition"]
    )
    assert [p.name for p in tmp_path.iterdir()] == ["store"]
    assert not (storage / "evil.txt").exists()


def test_missing_file_field_is_422(client: TestClient) -> None:
    assert client.post("/files", data={"other": "x"}).status_code == 422
