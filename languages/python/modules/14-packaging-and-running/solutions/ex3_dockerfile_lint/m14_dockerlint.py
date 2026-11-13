"""A tiny Dockerfile and .dockerignore checker (Docker is not needed: it only reads text)."""

import re
from dataclasses import dataclass

_PIP_INSTALL = re.compile(r"\bpip3?\s+install\b")


@dataclass(slots=True, frozen=True)
class Finding:
    rule: str
    line: int
    message: str


@dataclass(slots=True, frozen=True)
class _Instruction:
    line: int
    keyword: str
    args: str


def _parse(text: str) -> list[_Instruction]:
    instructions: list[_Instruction] = []
    start = 0
    parts: list[str] = []
    for number, raw in enumerate(text.splitlines(), 1):
        stripped = raw.strip()
        if stripped.startswith("#") or (not stripped and not parts):
            continue
        if not parts:
            start = number
        if stripped.endswith("\\"):
            parts.append(stripped[:-1].strip())
            continue
        parts.append(stripped)
        keyword, _, args = " ".join(p for p in parts if p).partition(" ")
        instructions.append(_Instruction(start, keyword.upper(), args.strip()))
        parts = []
    if parts:
        keyword, _, args = " ".join(p for p in parts if p).partition(" ")
        instructions.append(_Instruction(start, keyword.upper(), args.strip()))
    return instructions


def _split_stages(instructions: list[_Instruction]) -> list[list[_Instruction]]:
    stages: list[list[_Instruction]] = []
    for instruction in instructions:
        if instruction.keyword == "FROM":
            stages.append([])
        if stages:
            stages[-1].append(instruction)
    return stages


def _image_and_name(args: str) -> tuple[str, str | None]:
    words = [w for w in args.split() if not w.startswith("--")]
    name = words[2].lower() if len(words) >= 3 and words[1].upper() == "AS" else None
    return (words[0] if words else ""), name


def _unpinned(image: str, earlier_stages: set[str]) -> bool:
    if image == "scratch" or "$" in image or "@" in image or image.lower() in earlier_stages:
        return False
    last_component = image.rsplit("/", 1)[-1]
    _, _, tag = last_component.partition(":")
    return tag in ("", "latest")


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
    stages = _split_stages(_parse(text))
    findings: list[Finding] = []
    if len(stages) < 2:
        findings.append(Finding("DL001", 1, "use a multi-stage build (at least two FROM)"))
    names: set[str] = set()
    for stage in stages:
        image, name = _image_and_name(stage[0].args)
        if _unpinned(image, names):
            findings.append(Finding("DL002", stage[0].line, f"pin a tag for {image!r}"))
        if name:
            names.add(name)
        findings += _lint_stage_commands(stage)
    if stages:
        findings += _lint_final_stage(stages[-1])
    return sorted(findings, key=lambda f: (f.line, f.rule))


def _lint_stage_commands(stage: list[_Instruction]) -> list[Finding]:
    findings: list[Finding] = []
    no_cache_env = False
    first_pip = next(
        (
            i
            for i, ins in enumerate(stage)
            if ins.keyword == "RUN" and _PIP_INSTALL.search(ins.args)
        ),
        None,
    )
    for index, ins in enumerate(stage):
        if ins.keyword == "ENV" and "PIP_NO_CACHE_DIR" in ins.args:
            no_cache_env = True
        elif ins.keyword == "RUN" and _PIP_INSTALL.search(ins.args):
            if not no_cache_env and "--no-cache-dir" not in ins.args:
                findings.append(Finding("DL004", ins.line, "use pip install --no-cache-dir"))
        elif ins.keyword in ("CMD", "ENTRYPOINT") and not ins.args.startswith("["):
            findings.append(Finding("DL008", ins.line, f"use the exec form for {ins.keyword}"))
        elif ins.keyword == "COPY" and first_pip is not None and index < first_pip:
            words = [w for w in ins.args.split() if not w.startswith("--")]
            if not any(w.startswith("--from") for w in ins.args.split()) and (
                {".", "./"} & set(words[:-1])
            ):
                findings.append(Finding("DL005", ins.line, "copy the requirements file first"))
    return findings


def _lint_final_stage(stage: list[_Instruction]) -> list[Finding]:
    line = stage[0].line
    findings: list[Finding] = []
    users = [ins.args.split(":")[0].strip() for ins in stage if ins.keyword == "USER"]
    if not users or users[-1] in ("root", "0"):
        findings.append(Finding("DL003", line, "run the final stage as a non-root USER"))
    if not any(ins.keyword == "EXPOSE" for ins in stage):
        findings.append(Finding("DL006", line, "document the port with EXPOSE"))
    checks = [i for i in stage if i.keyword == "HEALTHCHECK" and i.args.upper() != "NONE"]
    if not checks:
        findings.append(Finding("DL007", line, "add a HEALTHCHECK"))
    return findings


def lint_dockerignore(text: str) -> list[Finding]:
    """Rule DI001 once per missing entry among `.git`, `.venv`, `.env`, `__pycache__`.

    An entry counts when a non-comment line equals it, or equals `**/<entry>`. Line is always 1.
    The message names the missing entry.
    """
    lines = {line.strip() for line in text.splitlines() if not line.strip().startswith("#")}
    return [
        Finding("DI001", 1, f"add {entry!r} to .dockerignore")
        for entry in (".git", ".venv", ".env", "__pycache__")
        if entry not in lines and f"**/{entry}" not in lines
    ]
