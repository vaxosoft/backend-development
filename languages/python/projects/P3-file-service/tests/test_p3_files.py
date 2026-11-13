import asyncio
import hashlib
import re
from collections.abc import AsyncIterator
from pathlib import Path
from typing import Any

import httpx
import pytest
from fastapi.testclient import TestClient
from p3_file_service.app import Config, create_app

Json = dict[str, Any]
BOUNDARY = "testboundary"
MULTIPART = {"content-type": f"multipart/form-data; boundary={BOUNDARY}"}
EMPTY_SHA256 = hashlib.sha256(b"").hexdigest()


class Service:
    def __init__(self, root: Path, max_size: int = 1024 * 1024) -> None:
        self.root = root
        self.store = root / "store"
        self.max_size = max_size
        self.client = TestClient(create_app(Config(storage_dir=self.store, max_size=max_size)))

    def upload(
        self, data: bytes, name: str = "report.txt", content_type: str | None = "text/plain"
    ) -> Json:
        r = self.client.post("/files", files={"file": (name, data, content_type)})
        assert r.status_code == 201, r.text
        body: Json = r.json()
        return body

    def stored_files(self) -> list[str]:
        return sorted(p.name for p in self.store.iterdir())


@pytest.fixture
def svc(tmp_path: Path) -> Service:
    return Service(tmp_path)


def raw_multipart(disposition: bytes, data: bytes, content_type: bytes | None = None) -> bytes:
    boundary = BOUNDARY.encode()
    headers = b"Content-Disposition: " + disposition + b"\r\n"
    if content_type:
        headers += b"Content-Type: " + content_type + b"\r\n"
    return b"--%b\r\n%b\r\n%b\r\n--%b--\r\n" % (boundary, headers, data, boundary)


def assert_error(response: httpx.Response, status: int, code: str) -> None:
    assert response.status_code == status, response.text
    assert response.headers["content-type"].startswith("application/json")
    error = response.json()["error"]
    assert error["code"] == code
    assert error["message"]


# --- upload ---


def test_upload_returns_201_with_id_name_size_hash_and_content_type(svc: Service) -> None:
    data = b"hello, files\n"
    info = svc.upload(data, name="report.txt", content_type="text/plain")
    assert re.fullmatch(r"[0-9a-f]{32}", info["id"])
    assert info == {
        "id": info["id"],
        "name": "report.txt",
        "size": len(data),
        "sha256": hashlib.sha256(data).hexdigest(),
        "content_type": "text/plain",
    }


def test_upload_without_a_part_content_type_defaults_to_octet_stream(svc: Service) -> None:
    body = raw_multipart(b'form-data; name="file"; filename="x.bin"', b"abc")
    r = svc.client.post("/files", content=body, headers=MULTIPART)
    assert r.status_code == 201
    assert r.json()["content_type"] == "application/octet-stream"


def test_upload_accepts_an_empty_file(svc: Service) -> None:
    info = svc.upload(b"")
    assert (info["size"], info["sha256"]) == (0, EMPTY_SHA256)


def test_upload_of_a_few_megabytes_keeps_every_byte(tmp_path: Path) -> None:
    svc = Service(tmp_path, max_size=8 * 1024 * 1024)
    data = bytes(range(256)) * (3 * 4096)
    info = svc.upload(data, content_type="application/octet-stream")
    assert info["size"] == len(data) == 3 * 1024 * 1024
    downloaded = svc.client.get(f"/files/{info['id']}")
    assert downloaded.content == data


def test_same_name_twice_gets_two_ids(svc: Service) -> None:
    assert svc.upload(b"1")["id"] != svc.upload(b"2")["id"]


def test_files_are_stored_under_generated_ids(svc: Service) -> None:
    info = svc.upload(b"data", name="my secret plans.txt")
    assert svc.stored_files() == [f"{info['id']}.bin", f"{info['id']}.json"]


@pytest.mark.parametrize(
    ("client_name", "expected"),
    [
        ("../../etc/passwd", "passwd"),
        ("..\\..\\windows\\win.ini", "win.ini"),
        ("/etc/shadow", "shadow"),
        ("a/b/c.txt", "c.txt"),
        ("..", "file"),
        ("...", "file"),
        ("dir/", "file"),
        ("na\u0000m\u0001e\u007f\t.txt", "name.txt"),
        ('say "hi".txt', "say _hi_.txt"),
        ("résumé.pdf", "résumé.pdf"),
    ],
)
def test_client_filename_is_sanitized_and_never_escapes_the_storage_dir(
    svc: Service, client_name: str, expected: str
) -> None:
    disposition = b'form-data; name="file"; filename="' + client_name.encode() + b'"'
    r = svc.client.post("/files", content=raw_multipart(disposition, b"x"), headers=MULTIPART)
    assert r.status_code == 201, r.text
    assert r.json()["name"] == expected
    assert [p.name for p in svc.root.iterdir()] == ["store"]
    assert all(re.fullmatch(r"[0-9a-f]{32}\.(bin|json)", n) for n in svc.stored_files())


def test_upload_exactly_at_the_limit_is_accepted(tmp_path: Path) -> None:
    svc = Service(tmp_path, max_size=1000)
    assert svc.upload(b"x" * 1000)["size"] == 1000


def test_upload_over_the_limit_is_413_and_leaves_nothing_behind(tmp_path: Path) -> None:
    svc = Service(tmp_path, max_size=1000)
    r = svc.client.post("/files", files={"file": ("big.bin", b"x" * 1001)})
    assert_error(r, 413, "payload_too_large")
    assert svc.stored_files() == []


def test_upload_stops_reading_the_body_once_the_limit_is_exceeded(tmp_path: Path) -> None:
    app = create_app(Config(storage_dir=tmp_path / "store", max_size=128 * 1024))
    sent = 0

    async def body() -> AsyncIterator[bytes]:
        nonlocal sent
        yield (
            f'--{BOUNDARY}\r\nContent-Disposition: form-data; name="file"; filename="big"\r\n\r\n'
        ).encode()
        for _ in range(500):  # 32 MB if it were read to the end
            sent += 1
            yield b"\0" * 64 * 1024
        yield f"\r\n--{BOUNDARY}--\r\n".encode()

    async def scenario() -> httpx.Response:
        transport = httpx.ASGITransport(app=app)
        async with httpx.AsyncClient(transport=transport, base_url="http://test") as client:
            return await client.post("/files", content=body(), headers=MULTIPART)

    response = asyncio.run(scenario())
    assert_error(response, 413, "payload_too_large")
    assert sent < 20


@pytest.mark.parametrize(
    ("content", "headers"),
    [
        (b"just text", {"content-type": "text/plain"}),
        (b"{}", {"content-type": "application/json"}),
        (raw_multipart(b'form-data; name="other"; filename="x.txt"', b"x"), MULTIPART),
        (raw_multipart(b'form-data; name="file"', b"not a file part"), MULTIPART),
        (b"--testboundary\r\nContent-Disposition: form-data; name=", MULTIPART),
    ],
)
def test_upload_without_a_usable_file_part_is_400(
    svc: Service, content: bytes, headers: dict[str, str]
) -> None:
    assert_error(svc.client.post("/files", content=content, headers=headers), 400, "bad_request")
    assert svc.stored_files() == []


# --- download ---


def test_download_streams_the_file_with_its_headers(svc: Service) -> None:
    data = b"%PDF-1.7 fake pdf bytes"
    info = svc.upload(data, name="report.pdf", content_type="application/pdf")
    r = svc.client.get(f"/files/{info['id']}")
    assert r.status_code == 200
    assert r.content == data
    assert r.headers["content-length"] == str(len(data))
    assert r.headers["content-type"] == "application/pdf"
    assert r.headers["content-disposition"] == 'attachment; filename="report.pdf"'
    assert r.headers["etag"] == f'"{info["sha256"]}"'


def test_download_of_a_non_ascii_or_spaced_name_keeps_a_quoted_filename(svc: Service) -> None:
    info = svc.upload(b"x", name="résumé final.txt")
    header = svc.client.get(f"/files/{info['id']}").headers["content-disposition"]
    assert header.startswith('attachment; filename="r_sum_ final.txt"')
    assert "filename*=UTF-8''r%C3%A9sum%C3%A9%20final.txt" in header


@pytest.mark.parametrize("variant", ["exact", "list", "weak", "star"])
def test_matching_if_none_match_returns_304_without_a_body(svc: Service, variant: str) -> None:
    info = svc.upload(b"cache me")
    etag = f'"{info["sha256"]}"'
    value = {"exact": etag, "list": f'"other", {etag}', "weak": f"W/{etag}", "star": "*"}[variant]
    r = svc.client.get(f"/files/{info['id']}", headers={"If-None-Match": value})
    assert r.status_code == 304
    assert r.content == b""
    assert r.headers["etag"] == etag


def test_non_matching_if_none_match_returns_the_file(svc: Service) -> None:
    info = svc.upload(b"cache me")
    r = svc.client.get(f"/files/{info['id']}", headers={"If-None-Match": '"stale"'})
    assert r.status_code == 200
    assert r.content == b"cache me"


@pytest.mark.parametrize(
    "file_id", ["0" * 32, "abc", "A" * 32, "0" * 31, "..", "%2e%2e%2fstore", "x.json"]
)
def test_unknown_or_malformed_id_is_404(svc: Service, file_id: str) -> None:
    assert_error(svc.client.get(f"/files/{file_id}"), 404, "not_found")
    assert_error(svc.client.delete(f"/files/{file_id}"), 404, "not_found")


def test_files_survive_a_restart(tmp_path: Path) -> None:
    info = Service(tmp_path).upload(b"still here")
    second = Service(tmp_path)
    assert second.client.get(f"/files/{info['id']}").content == b"still here"


# --- delete ---


def test_delete_returns_204_and_removes_the_file(svc: Service) -> None:
    info = svc.upload(b"bye")
    r = svc.client.delete(f"/files/{info['id']}")
    assert r.status_code == 204
    assert r.content == b""
    assert svc.stored_files() == []
    assert_error(svc.client.get(f"/files/{info['id']}"), 404, "not_found")
    assert_error(svc.client.delete(f"/files/{info['id']}"), 404, "not_found")
