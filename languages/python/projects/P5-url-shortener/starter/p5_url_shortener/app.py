from fastapi import FastAPI

from .config import Config
from .errors import install_error_handlers


def create_app(config: Config) -> FastAPI:
    app = FastAPI(title="URL shortener")
    install_error_handlers(app)
    # TODO: open the database (db.connect) and build Store, RateLimiter
    # TODO: install_access_log(app, config)
    # TODO: routes: POST /auth/register, POST /auth/login, POST /links, GET /links,
    #       DELETE /links/{slug}, GET /{slug} (register GET /{slug} last)
    # TODO: a `current_user` dependency (Bearer token -> user id, 401 otherwise)
    _ = config
    return app
