"""Build metadata for a service: version, commit and user agent, injected at build time."""

import re
import tomllib
from collections.abc import Mapping
from dataclasses import dataclass

_VERSION = re.compile(r"v?(\d+)\.(\d+)\.(\d+)")


@dataclass(slots=True, frozen=True)
class BuildInfo:
    version: str
    commit: str

    @property
    def short_commit(self) -> str:
        """First 7 characters of the commit, or the whole value (e.g. "unknown") if shorter."""
        return self.commit[:7]

    def user_agent(self) -> str:
        """`task-tracker/<version>` (used by outgoing HTTP requests)."""
        return f"task-tracker/{self.version}"

    def as_dict(self) -> dict[str, str]:
        """Payload for `GET /version`: keys `version` and `commit` (the short commit)."""
        return {"version": self.version, "commit": self.short_commit}


def load_build_info(environ: Mapping[str, str]) -> BuildInfo:
    """Read APP_VERSION (default "dev") and GIT_COMMIT (default "unknown").

    Surrounding whitespace is stripped; an empty or blank value counts as unset.
    """
    version = environ.get("APP_VERSION", "").strip() or "dev"
    commit = environ.get("GIT_COMMIT", "").strip() or "unknown"
    return BuildInfo(version=version, commit=commit)


def read_project_version(pyproject_text: str) -> str:
    """Return `[project] version` from the text of a pyproject.toml.

    Raise ValueError when the file has no static version (missing, or declared `dynamic`).
    """
    project = tomllib.loads(pyproject_text).get("project", {})
    version = project.get("version")
    if not isinstance(version, str):
        raise ValueError("pyproject.toml has no static [project] version")
    return version


def parse_version(text: str) -> tuple[int, int, int]:
    """Parse `1.4.2` or `v1.4.2` into (1, 4, 2) so versions compare numerically.

    Raise ValueError for anything that is not exactly three dot-separated integers.
    """
    match = _VERSION.fullmatch(text)
    if match is None:
        raise ValueError(f"not a MAJOR.MINOR.PATCH version: {text!r}")
    major, minor, patch = match.groups()
    return int(major), int(minor), int(patch)
