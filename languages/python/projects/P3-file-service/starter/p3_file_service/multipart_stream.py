"""Incremental multipart reader: hands the bytes of one file part to the caller as they arrive.

Starlette's `request.form()` parses the complete body before your handler runs. This module
instead feeds `request.stream()` into python-multipart's low-level parser chunk by chunk, so the
handler can hash, count and write to disk while the upload is still in flight, and can stop
reading (and answer 413) as soon as a limit is exceeded.
"""

from collections.abc import AsyncIterator
from dataclasses import dataclass, field

from python_multipart.exceptions import FormParserError
from python_multipart.multipart import MultipartParser, parse_options_header
from starlette.requests import Request


class MalformedUpload(Exception):
    """The body is not a well-formed multipart/form-data message."""


@dataclass(slots=True)
class _Progress:
    headers: dict[bytes, bytes] = field(default_factory=dict)
    name: list[bytes] = field(default_factory=list)
    value: list[bytes] = field(default_factory=list)
    active: bool = False  # inside the part we are looking for
    found: bool = False  # its headers have been seen
    finished: bool = False  # its last byte has been seen
    filename: str = ""
    content_type: str = "application/octet-stream"
    pending: list[bytes] = field(default_factory=list)


class FilePart:
    """The file part of an upload: `filename`, `content_type` and `chunks()` with its bytes."""

    def __init__(self, progress: _Progress, body: AsyncIterator[bytes], parser: MultipartParser):
        self._progress = progress
        self._body = body
        self._parser = parser
        self.filename = progress.filename
        self.content_type = progress.content_type

    async def chunks(self) -> AsyncIterator[bytes]:
        progress = self._progress
        while True:
            while progress.pending:
                yield progress.pending.pop(0)
            if progress.finished:
                return
            if not await _feed(self._body, self._parser):
                raise MalformedUpload("the body ended inside the file part")


async def _feed(body: AsyncIterator[bytes], parser: MultipartParser) -> bool:
    """Pass the next chunk of the request body to the parser. False when the body is over."""
    try:
        chunk = await anext(body)
    except StopAsyncIteration:
        return False
    try:
        parser.write(chunk)
    except FormParserError as err:
        raise MalformedUpload(str(err)) from err
    return True


async def open_file_part(request: Request, field_name: str) -> FilePart | None:
    """Read just far enough to see the headers of the part called `field_name`.

    Returns None when the body ends without such a file part. Raises MalformedUpload when the
    request is not multipart/form-data or cannot be parsed.
    """
    content_type, options = parse_options_header(request.headers.get("content-type"))
    boundary = options.get(b"boundary")
    if content_type != b"multipart/form-data" or not boundary:
        raise MalformedUpload("expected a multipart/form-data body")

    progress = _Progress()

    def on_part_begin() -> None:
        progress.headers = {}
        progress.active = False

    def on_header_field(data: bytes, start: int, end: int) -> None:
        progress.name.append(data[start:end])

    def on_header_value(data: bytes, start: int, end: int) -> None:
        progress.value.append(data[start:end])

    def on_header_end() -> None:
        progress.headers[b"".join(progress.name).lower()] = b"".join(progress.value)
        progress.name.clear()
        progress.value.clear()

    def on_headers_finished() -> None:
        _, options = parse_options_header(progress.headers.get(b"content-disposition"))
        wanted = (
            not progress.found
            and options.get(b"name") == field_name.encode()
            and b"filename" in options
        )
        if wanted:
            progress.active = progress.found = True
            progress.filename = options[b"filename"].decode("utf-8", "replace")
            declared = progress.headers.get(b"content-type")
            if declared:
                progress.content_type = declared.decode("latin-1")

    def on_part_data(data: bytes, start: int, end: int) -> None:
        if progress.active:
            progress.pending.append(data[start:end])

    def on_part_end() -> None:
        if progress.active:
            progress.active = False
            progress.finished = True

    parser = MultipartParser(
        boundary,
        {
            "on_part_begin": on_part_begin,
            "on_header_field": on_header_field,
            "on_header_value": on_header_value,
            "on_header_end": on_header_end,
            "on_headers_finished": on_headers_finished,
            "on_part_data": on_part_data,
            "on_part_end": on_part_end,
        },
    )
    body = aiter(request.stream())
    while not progress.found:
        if not await _feed(body, parser):
            return None
    return FilePart(progress, body, parser)
