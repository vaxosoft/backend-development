"""pathlib, text versus bytes, explicit encodings and open modes."""

import tempfile
from pathlib import Path


def main() -> None:
    with tempfile.TemporaryDirectory() as tmp:
        root = Path(tmp)

        # Path objects compose with `/`; mkdir(parents=True, exist_ok=True) is the safe idiom
        notes = root / "notes" / "todo.txt"
        notes.parent.mkdir(parents=True, exist_ok=True)
        print(notes.name, notes.stem, notes.suffix, notes.parent.name)

        # Text I/O always names its encoding; the default depends on the machine
        notes.write_text("café\nnaïve\n", encoding="utf-8")
        text = notes.read_text(encoding="utf-8")
        raw = notes.read_bytes()
        print("characters:", len(text), "bytes:", len(raw))
        print("decoded with the wrong codec:", repr(raw.decode("latin-1")))

        # Modes: "a" appends, "x" refuses to overwrite
        with notes.open("a", encoding="utf-8") as handle:
            handle.write("third line\n")
        print("lines:", notes.read_text(encoding="utf-8").splitlines())
        try:
            with notes.open("x", encoding="utf-8"):
                pass
        except FileExistsError:
            print("mode 'x' on an existing file: FileExistsError")

        # Directories
        (root / "notes" / "done.txt").write_text("", encoding="utf-8")
        (root / "data.bin").write_bytes(bytes(range(4)))
        print(sorted(p.relative_to(root).as_posix() for p in root.rglob("*")))
        print("*.txt:", sorted(p.name for p in (root / "notes").glob("*.txt")))
        print("size:", (root / "data.bin").stat().st_size, "is_file:", notes.is_file())

        # Missing files raise; check with exceptions rather than exists() then open (a race)
        try:
            (root / "missing.txt").read_text(encoding="utf-8")
        except FileNotFoundError as exc:
            print("raised:", type(exc).__name__)


if __name__ == "__main__":
    main()
