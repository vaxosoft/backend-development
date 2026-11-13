"""Request parsing and response writing: forms, uploads, cookies, headers, streaming, negotiation.

Run:  python 03_requests_and_responses.py   (ADDR=host:port to change the address)
"""

import hashlib
from collections.abc import Iterator
from typing import Annotated

from fastapi import Cookie, FastAPI, Form, Request, Response, UploadFile
from fastapi.responses import JSONResponse, PlainTextResponse, StreamingResponse
from m02_helpers import serve

app = FastAPI()

CHUNK = 64 * 1024


@app.post("/login")
def login(
    response: Response,
    username: Annotated[str, Form()],
    password: Annotated[str, Form()],
) -> dict[str, str]:
    response.set_cookie("session", f"s-{username}", httponly=True, samesite="lax", max_age=3600)
    return {"user": username}


@app.get("/me")
def me(session: Annotated[str | None, Cookie()] = None) -> dict[str, str | None]:
    return {"session": session}


@app.post("/upload", status_code=201)
async def upload(file: UploadFile) -> dict[str, object]:
    digest = hashlib.sha256()
    size = 0
    while chunk := await file.read(CHUNK):
        digest.update(chunk)
        size += len(chunk)
    return {
        "filename": file.filename,
        "content_type": file.content_type,
        "size": size,
        "sha256": digest.hexdigest(),
    }


@app.post("/tasks", status_code=201)
def create(response: Response) -> dict[str, int]:
    response.headers["Location"] = "/tasks/42"
    response.headers["X-Demo"] = "created"
    return {"id": 42}


@app.get("/report", response_model=None)
def report(request: Request) -> Response:
    rows = [{"id": 1, "title": "Write docs"}, {"id": 2, "title": "Ship"}]
    accept = request.headers.get("accept", "*/*")
    if "text/csv" in accept:
        body = "id,title\n" + "".join(f"{r['id']},{r['title']}\n" for r in rows)
        return PlainTextResponse(body, media_type="text/csv")
    if "application/json" in accept or "*/*" in accept:
        return JSONResponse(rows)
    return JSONResponse(
        {"error": {"code": "not_acceptable", "message": "use application/json or text/csv"}},
        status_code=406,
    )


def numbers(count: int) -> Iterator[str]:
    for n in range(count):
        yield f"line {n}\n"


@app.get("/stream")
def stream(count: int = 5) -> StreamingResponse:
    return StreamingResponse(numbers(count), media_type="text/plain")


if __name__ == "__main__":
    serve(app)
