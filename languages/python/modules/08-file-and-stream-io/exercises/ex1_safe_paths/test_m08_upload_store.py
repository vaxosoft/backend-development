from pathlib import Path

import pytest
from m08_upload_store import UnknownFileError, UploadStore, display_name

FIXED_ID = "0123456789abcdef0123456789abcdef"


@pytest.fixture
def store(tmp_path: Path) -> UploadStore:
    base = tmp_path / "uploads"
    base.mkdir()
    (tmp_path / "secret.txt").write_text("top secret", encoding="utf-8")
    return UploadStore(base, new_id=lambda: FIXED_ID)


@pytest.mark.parametrize(
    ("raw", "expected"),
    [
        ("report.pdf", "report.pdf"),
        ("../../etc/passwd", "passwd"),
        ("C:\\temp\\evil.exe", "evil.exe"),
        ("my report (final).pdf", "my report (final).pdf"),
        ("bad\x00name\n.txt", "bad_name_.txt"),
        ("  .hidden  ", "hidden"),
        ("..", "file"),
        ("", "file"),
        ("dir/", "file"),
    ],
)
def test_display_name_is_a_harmless_leaf(raw: str, expected: str) -> None:
    assert display_name(raw) == expected


def test_save_stores_bytes_under_the_generated_id(store: UploadStore, tmp_path: Path) -> None:
    stored = store.save("notes.txt", b"hello")

    assert (stored.id, stored.name, stored.size) == (FIXED_ID, "notes.txt", 5)
    assert (tmp_path / "uploads" / FIXED_ID).read_bytes() == b"hello"
    assert store.read(FIXED_ID) == b"hello"


def test_hostile_filename_cannot_escape_the_storage_directory(
    store: UploadStore, tmp_path: Path
) -> None:
    stored = store.save("../../outside.txt", b"x")

    assert stored.name == "outside.txt"
    assert sorted(p.name for p in tmp_path.rglob("*") if p.is_file()) == sorted(
        ["secret.txt", FIXED_ID]
    )
    assert (tmp_path / "uploads" / FIXED_ID).is_file()


@pytest.mark.parametrize(
    "bad_id", ["../secret.txt", "..", "/etc/passwd", "", "ABCDEF0123456789ABCDEF0123456789", "abc"]
)
def test_malformed_ids_are_unknown_not_a_path_lookup(store: UploadStore, bad_id: str) -> None:
    with pytest.raises(UnknownFileError):
        store.read(bad_id)
    with pytest.raises(UnknownFileError):
        store.delete(bad_id)


def test_unknown_but_well_formed_id(store: UploadStore) -> None:
    with pytest.raises(UnknownFileError):
        store.read("f" * 32)


def test_delete_removes_the_file_once(store: UploadStore, tmp_path: Path) -> None:
    store.save("a.txt", b"a")

    store.delete(FIXED_ID)

    assert not (tmp_path / "uploads" / FIXED_ID).exists()
    with pytest.raises(UnknownFileError):
        store.delete(FIXED_ID)
