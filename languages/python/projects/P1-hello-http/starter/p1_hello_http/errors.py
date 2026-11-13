from fastapi.responses import JSONResponse


def error_response(status: int, code: str, message: str) -> JSONResponse:
    """Build the shared error envelope: {"error": {"code": ..., "message": ...}}."""
    return JSONResponse(
        status_code=status,
        content={"error": {"code": code, "message": message}},
    )
