"""Imports every pinned dependency once so a broken install fails loudly. Not a lesson."""

import fastapi
import httpx
import jwt
import psycopg
import pydantic
import pydantic_settings
import sqlalchemy
import uvicorn
import yaml

print(
    fastapi.__version__,
    httpx.__version__,
    jwt.__version__,
    psycopg.__version__,
    pydantic.__version__,
    pydantic_settings.__name__,
    sqlalchemy.__version__,
    uvicorn.__version__,
    yaml.__version__,
)
