"""Build metadata for a service: version, commit and user agent, injected at build time."""

from collections.abc import Mapping
from dataclasses import dataclass


@dataclass(slots=True, frozen=True)
class BuildInfo:
    version: str
    commit: str

    @property
    def short_commit(self) -> str:
        """First 7 characters of the commit, or the whole value (e.g. "unknown") if shorter."""
        raise NotImplementedError

    def user_agent(self) -> str:
        """`task-tracker/<version>` (used by outgoing HTTP requests)."""
        raise NotImplementedError

    def as_dict(self) -> dict[str, str]:
        """Payload for `GET /version`: keys `version` and `commit` (the short commit)."""
        raise NotImplementedError


def load_build_info(environ: Mapping[str, str]) -> BuildInfo:
    """Read APP_VERSION (default "dev") and GIT_COMMIT (default "unknown").

    Surrounding whitespace is stripped; an empty or blank value counts as unset.
    """
    raise NotImplementedError


def read_project_version(pyproject_text: str) -> str:
    """Return `[project] version` from the text of a pyproject.toml.

    Raise ValueError when the file has no static version (missing, or declared `dynamic`).
    """
    raise NotImplementedError


def parse_version(text: str) -> tuple[int, int, int]:
    """Parse `1.4.2` or `v1.4.2` into (1, 4, 2) so versions compare numerically.

    Raise ValueError for anything that is not exactly three dot-separated integers.
    """
    raise NotImplementedError
