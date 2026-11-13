"""Exercise 1: model a resource with the right verbs and status codes."""

from itertools import count

from fastapi import FastAPI, Request, Response
from fastapi.responses import JSONResponse
from pydantic import BaseModel


class NoteIn(BaseModel):
    text: str


class Note(BaseModel):
    id: int
    text: str


class NotFound(Exception):
    def __init__(self, note_id: int) -> None:
        super().__init__(f"note {note_id} not found")


def create_app() -> FastAPI:
    """Routes to add:

    POST   /notes       body {"text": ...} -> 201, Location: /notes/<id>, body {"id", "text"}
    GET    /notes       -> 200, list of notes in creation order
    GET    /notes/{id}  -> 200 note, or 404 for an unknown id
    PUT    /notes/{id}  -> 200 with the replaced note, or 404
    DELETE /notes/{id}  -> 204 with an EMPTY body, or 404 (so a second DELETE is a 404)

    404 body: {"error": {"code": "not_found", "message": "note <id> not found"}}
    Raise `NotFound(id)` from handlers; the exception handler below turns it into that body.
    Ids start at 1 and are never reused.
    """
    app = FastAPI()
    notes: dict[int, Note] = {}
    ids = count(1)

    @app.exception_handler(NotFound)
    async def on_not_found(request: Request, exc: NotFound) -> JSONResponse:
        return JSONResponse(
            {"error": {"code": "not_found", "message": str(exc)}},
            status_code=404,
        )

    def find(note_id: int) -> Note:
        if note_id not in notes:
            raise NotFound(note_id)
        return notes[note_id]

    @app.post("/notes", status_code=201)
    def create(data: NoteIn, response: Response) -> Note:
        note = Note(id=next(ids), text=data.text)
        notes[note.id] = note
        response.headers["Location"] = f"/notes/{note.id}"
        return note

    @app.get("/notes")
    def list_notes() -> list[Note]:
        return list(notes.values())

    @app.get("/notes/{note_id}")
    def read(note_id: int) -> Note:
        return find(note_id)

    @app.put("/notes/{note_id}")
    def replace(note_id: int, data: NoteIn) -> Note:
        find(note_id)
        notes[note_id] = Note(id=note_id, text=data.text)
        return notes[note_id]

    @app.delete("/notes/{note_id}", status_code=204)
    def delete(note_id: int) -> None:
        find(note_id)
        del notes[note_id]

    return app
