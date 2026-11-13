"""A tiny Dockerfile and .dockerignore checker (Docker is not needed: it only reads text)."""

from dataclasses import dataclass


@dataclass(slots=True, frozen=True)
class Finding:
    rule: str
    line: int
    message: str


def lint_dockerfile(text: str) -> list[Finding]:
    """Check a Dockerfile and return findings sorted by (line, rule). A good file yields [].

    Parse first: skip blank and `#` comment lines, join lines ending in `\\`, ignore instruction
    case. `line` is the 1-based line where the instruction starts (1 for whole-file findings).

    - DL001: fewer than two FROM instructions (not multi-stage)
    - DL002: base image without a pinned tag (`python`, `python:latest`); a digest, `scratch`,
      an earlier stage name and `$VARIABLE` images are fine
    - DL003: the final stage does not end as a non-root user (no USER, or root / 0)
    - DL004: `pip install` without `--no-cache-dir`, unless an earlier `ENV` in the stage
      sets PIP_NO_CACHE_DIR
    - DL005: in a stage, `COPY . ...` comes before the stage's first `pip install`
      (`COPY --from=...` is ignored)
    - DL006: the final stage has no EXPOSE
    - DL007: the final stage has no HEALTHCHECK (`HEALTHCHECK NONE` counts as none)
    - DL008: CMD or ENTRYPOINT in shell form instead of the JSON exec form

    For DL003, DL006 and DL007 the line is that of the final FROM. A `USER` in an earlier stage
    does not count, and `user:group` forms are judged by the user part.
    """
    raise NotImplementedError


def lint_dockerignore(text: str) -> list[Finding]:
    """Rule DI001 once per missing entry among `.git`, `.venv`, `.env`, `__pycache__`.

    An entry counts when a non-comment line equals it, or equals `**/<entry>`. Line is always 1.
    The message names the missing entry.
    """
    raise NotImplementedError
