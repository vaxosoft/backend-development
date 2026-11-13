"""Domain records and the request/response models."""

import re
from dataclasses import dataclass
from typing import Annotated
from urllib.parse import urlsplit

from pydantic import AfterValidator, BaseModel, Field

SLUG_PATTERN = r"^[A-Za-z0-9_-]{3,32}$"
MAX_URL_LENGTH = 2048


@dataclass(frozen=True, slots=True)
class User:
    id: int
    email: str
    password_hash: str


@dataclass(frozen=True, slots=True)
class Link:
    slug: str
    url: str
    owner_id: int
    hits: int
    created_at: str


def _check_email(value: str) -> str:
    email = value.strip().lower()
    local, at, domain = email.partition("@")
    if not (local and at and domain):
        raise ValueError('must be an email address containing "@"')
    return email


def _check_url(value: str) -> str:
    parts = urlsplit(value.strip())
    if parts.scheme not in ("http", "https") or not parts.hostname:
        raise ValueError("must be an absolute http or https URL")
    return value.strip()


Email = Annotated[str, Field(max_length=254), AfterValidator(_check_email)]
HttpUrl = Annotated[str, Field(max_length=MAX_URL_LENGTH), AfterValidator(_check_url)]
Slug = Annotated[str, Field(pattern=re.compile(SLUG_PATTERN))]


class RegisterRequest(BaseModel):
    email: Email
    password: Annotated[str, Field(min_length=8, max_length=128)]


class LoginRequest(BaseModel):
    email: str
    password: str


class LinkCreate(BaseModel):
    url: HttpUrl
    slug: Slug | None = None
