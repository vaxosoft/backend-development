"""Domain records and the request/response models."""

from dataclasses import dataclass

from pydantic import BaseModel


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


class RegisterRequest(BaseModel):
    # TODO: email must contain "@" (<= 254 chars); password must be 8-128 characters
    email: str
    password: str


class LoginRequest(BaseModel):
    email: str
    password: str


class LinkCreate(BaseModel):
    # TODO: url must be an absolute http/https URL; slug (optional) is 3-32 chars of [A-Za-z0-9_-]
    url: str
    slug: str | None = None
