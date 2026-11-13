"""Never trust a user-supplied path; read bundled assets with importlib.resources."""

import re
import tempfile
from importlib import resources
from pathlib import Path


class PathTraversalError(ValueError):
    pass


def safe_join(base: Path, user_path: str) -> Path:
    """Resolve `user_path` under `base`, refusing anything that ends up outside it."""
    root = base.resolve()
    candidate = (root / user_path).resolve()  # resolve() collapses `..` and follows symlinks
    if not candidate.is_relative_to(root):
        raise PathTraversalError(f"{user_path!r} escapes the storage directory")
    return candidate


def sanitize_filename(name: str) -> str:
    """Keep only a harmless display name; it is never used as a path."""
    leaf = re.split(r"[\\/]", name)[-1]
    cleaned = re.sub(r"[^\w.\- ]", "_", leaf).strip(" .")
    return cleaned or "file"


def main() -> None:
    with tempfile.TemporaryDirectory() as tmp:
        storage = Path(tmp) / "uploads"
        storage.mkdir()
        outside = Path(tmp) / "secret.txt"
        outside.write_text("secret", encoding="utf-8")
        (storage / "link").symlink_to(outside)

        for requested in (
            "report.txt",
            "sub/../report.txt",
            "../secret.txt",
            "/etc/passwd",
            "link",
        ):
            try:
                result = safe_join(storage, requested)
                print(f"{requested:20} ok -> {result.relative_to(storage.resolve())}")
            except PathTraversalError:
                print(f"{requested:20} rejected")

    for name in (
        "../../etc/passwd",
        "C:\\temp\\evil.exe",
        "report (final).pdf",
        "..",
        "bad\x00name.txt",
    ):
        print(f"{name!r:24} -> {sanitize_filename(name)!r}")

    # Embedded assets: located relative to a package, so they work from any directory
    asset = resources.files("m08_assets").joinpath("welcome.txt")
    print(asset.read_text(encoding="utf-8").splitlines()[0])


if __name__ == "__main__":
    main()
