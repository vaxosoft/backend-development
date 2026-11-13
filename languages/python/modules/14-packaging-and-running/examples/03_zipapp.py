"""Build a self-contained, runnable archive with zipapp and run it like a binary."""

import subprocess
import sys
import tempfile
import zipapp
import zipfile
from pathlib import Path

MAIN = """\
import sys

from greeter import greeting


def main() -> None:
    print(greeting(sys.argv[1] if len(sys.argv) > 1 else "world"))
"""

LIB = """\
def greeting(name: str) -> str:
    return f"Hello, {name}!"
"""

with tempfile.TemporaryDirectory() as tmp:
    source = Path(tmp) / "app"
    source.mkdir()
    (source / "greeter.py").write_text(LIB, encoding="utf-8")
    (source / "cli.py").write_text(MAIN, encoding="utf-8")

    archive = Path(tmp) / "greeter.pyz"
    zipapp.create_archive(
        source, archive, interpreter="/usr/bin/env python3", main="cli:main", compressed=True
    )
    shebang = archive.read_bytes().split(b"\n", 1)[0].decode()
    print("archive:", archive.name, "| shebang:", shebang)
    print("entries:", sorted(zipfile.ZipFile(archive).namelist()))

    for extra in ([], ["Task Tracker"]):
        result = subprocess.run(
            [sys.executable, str(archive), *extra], capture_output=True, text=True, check=True
        )
        print("run:", result.stdout.strip())
