"""Repository hygiene: plain text/YAML checks of the packaging files. Docker is never run."""

import re
from pathlib import Path
from typing import Any

import yaml

STARTER = Path(__file__).resolve().parent.parent / "starter"


def read(relative: str) -> str:
    path = STARTER / relative
    assert path.is_file(), f"{relative} is missing from starter/"
    return path.read_text(encoding="utf-8")


def dockerfile_stages() -> list[list[tuple[str, str]]]:
    """Split the Dockerfile into stages of (INSTRUCTION, arguments), joining `\\` continuations."""
    text = re.sub(r"\\\s*\n", " ", read("Dockerfile"))
    stages: list[list[tuple[str, str]]] = []
    for line in text.splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        instruction, _, arguments = line.partition(" ")
        if instruction.upper() == "FROM":
            stages.append([])
        assert stages, "the first instruction must be FROM"
        stages[-1].append((instruction.upper(), arguments.strip()))
    return stages


def instructions(stage: list[tuple[str, str]], name: str) -> list[str]:
    return [arguments for instruction, arguments in stage if instruction == name]


def workflow() -> dict[str, Any]:
    document = yaml.safe_load(read(".github/workflows/ci.yml"))
    assert isinstance(document, dict)
    if True in document:  # YAML 1.1 turns the key `on` into the boolean True
        document["on"] = document.pop(True)
    return document


def workflow_steps() -> list[dict[str, Any]]:
    return [step for job in workflow()["jobs"].values() for step in job["steps"]]


def test_dockerfile_is_multi_stage_with_a_named_build_stage() -> None:
    stages = dockerfile_stages()

    assert len(stages) >= 2
    assert re.search(r"\bAS\s+\w+", instructions(stages[0], "FROM")[0], re.IGNORECASE)


def test_final_stage_copies_the_build_output_and_does_not_install_anything() -> None:
    stages = dockerfile_stages()
    build, final = stages[0], stages[-1]

    assert any("pip install" in command for command in instructions(build, "RUN"))
    assert any(arg.startswith("--from=") for arg in instructions(final, "COPY"))
    assert not any(
        re.search(r"pip install|apt-get|apk add", command) for command in instructions(final, "RUN")
    )


def test_final_stage_runs_as_a_non_root_user() -> None:
    final = dockerfile_stages()[-1]

    users = instructions(final, "USER")
    assert users, "the final stage needs a USER instruction"
    assert users[-1].split(":")[0] not in {"root", "0"}


def test_dockerfile_exposes_a_port_and_declares_a_healthcheck_and_a_command() -> None:
    final = dockerfile_stages()[-1]

    assert instructions(final, "EXPOSE")
    assert instructions(final, "HEALTHCHECK")
    assert instructions(final, "CMD") or instructions(final, "ENTRYPOINT")


def test_base_images_are_pinned_to_a_version_tag() -> None:
    for stage in dockerfile_stages():
        image = instructions(stage, "FROM")[0].split()[0]
        tag = image.partition(":")[2]
        assert tag not in {"", "latest"}, f"unpinned base image: {image}"


def test_dockerignore_keeps_secrets_and_junk_out_of_the_build_context() -> None:
    entries = {
        line.strip() for line in read(".dockerignore").splitlines() if not line.startswith("#")
    }

    assert {".git", ".venv", ".env", "__pycache__/"} <= entries


def test_ci_runs_on_push_and_pull_request() -> None:
    triggers = workflow()["on"]

    assert {"push", "pull_request"} <= set(triggers)


def test_ci_checks_formatting_lints_and_tests_in_that_order() -> None:
    commands = [step["run"] for step in workflow_steps() if "run" in step]

    def position(fragment: str) -> int:
        matches = [i for i, command in enumerate(commands) if fragment in command]
        assert matches, f"no CI step runs {fragment!r}"
        return matches[0]

    install = position("pip install")
    assert install < position("ruff format --check") < position("ruff check") < position("pytest")


def test_ci_uses_the_same_python_version_as_the_image() -> None:
    setups = [
        str(step["with"]["python-version"])
        for step in workflow_steps()
        if str(step.get("uses", "")).startswith("actions/setup-python")
    ]
    image_version = re.search(
        r"python:(\d+\.\d+)", instructions(dockerfile_stages()[-1], "FROM")[0]
    )

    assert image_version is not None
    assert setups == [image_version.group(1)]
