"""Exercise 1: bind path, query and header parameters from type hints."""

from fastapi import FastAPI


def create_app() -> FastAPI:
    """Build the app. Routes to add (see the tests for exact JSON shapes):

    GET /hello                      -> text/plain "Hello, world!"
    GET /hello/{name}               -> text/plain "Hello, {name}!"
    GET /add?a=1&b=2                -> {"sum": 3}            (a and b are required ints)
    GET /users/{user_id}/tasks/{task_id}
                                    -> {"user_id": 3, "task_id": 7}   (task_id >= 1)
    GET /search?tag=x&tag=y&limit=5 -> {"tags": ["x", "y"], "limit": 5}
                                       (tag repeatable, optional; limit 1-50, default 10)
    GET /agent                      -> {"user_agent": "<User-Agent header>"}
    Invalid parameters must produce FastAPI's default 422.
    """
    app = FastAPI()
    return app
