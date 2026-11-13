"""Compression (gzip, zip), encodings (base64, hex) and hashing."""

import base64
import gzip
import hashlib
import io
import zipfile


def main() -> None:
    payload = b'{"title": "Write docs", "status": "todo"}\n' * 200

    # gzip: one compressed stream; repetitive JSON shrinks a lot
    packed = gzip.compress(payload, mtime=0)  # mtime=0 keeps the header reproducible
    print(
        "gzip smaller:",
        len(packed) < len(payload) // 10,
        "| round trip ok:",
        gzip.decompress(packed) == payload,
    )

    # text mode wraps the compressed stream in a TextIOWrapper
    buffer = io.BytesIO()
    with gzip.open(buffer, "wt", encoding="utf-8") as out:
        out.write("one\ntwo\n")
    with gzip.open(io.BytesIO(buffer.getvalue()), "rt", encoding="utf-8") as lines:
        print("gzip text lines:", [line.strip() for line in lines])

    # zip: an archive of named members
    archive = io.BytesIO()
    with zipfile.ZipFile(archive, "w", compression=zipfile.ZIP_DEFLATED) as zf:
        zf.writestr("tasks.json", '[{"id": 1}]')
        zf.writestr("notes/readme.txt", "hello")
    with zipfile.ZipFile(archive) as zf:
        print("zip members:", zf.namelist())
        print("read member:", zf.read("tasks.json").decode("utf-8"))

    # encodings turn bytes into text safely; they are NOT encryption
    token = bytes([251, 255, 190, 250, 0])
    print("hex:      ", token.hex(), bytes.fromhex(token.hex()) == token)
    print("base64:   ", base64.b64encode(token).decode("ascii"))
    print("urlsafe:  ", base64.urlsafe_b64encode(token).decode("ascii"))
    print("decoded:  ", base64.b64decode("+/+++gA=") == token)

    # hashing: a fixed-size fingerprint; compare digests, not contents
    print("sha256:   ", hashlib.sha256(b"hello").hexdigest())
    print(
        "md5 only for checksums, not security:",
        hashlib.md5(b"hello", usedforsecurity=False).hexdigest(),
    )


if __name__ == "__main__":
    main()
