from pathlib import Path

import pytest
from m14_dockerlint import lint_dockerfile, lint_dockerignore

GOOD = """\
# syntax=docker/dockerfile:1
FROM python:3.14-slim AS build
RUN python -m venv /opt/venv
ENV PATH="/opt/venv/bin:$PATH"
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt
COPY . .

FROM python:3.14-slim
RUN useradd --system app
COPY --from=build /opt/venv /opt/venv
COPY --from=build /src /app
USER app
EXPOSE 8080
HEALTHCHECK CMD ["python", "-c", "print('ok')"]
CMD ["python", "service.py"]
"""

EXAMPLES = Path(__file__).resolve().parents[2] / "examples"


def rules(text: str) -> list[str]:
    return [f.rule for f in lint_dockerfile(text)]


def test_a_good_dockerfile_has_no_findings() -> None:
    assert lint_dockerfile(GOOD) == []


def test_the_example_dockerfile_and_dockerignore_are_clean() -> None:
    dockerfile = (EXAMPLES / "deploy" / "Dockerfile").read_text(encoding="utf-8")
    dockerignore = (EXAMPLES / ".dockerignore").read_text(encoding="utf-8")
    assert lint_dockerfile(dockerfile) == []
    assert lint_dockerignore(dockerignore) == []


def test_single_stage_build() -> None:
    text = "FROM python:3.14-slim\nUSER app\nEXPOSE 80\nHEALTHCHECK CMD true\n"
    assert rules(text) == ["DL001"]


def test_empty_file_is_not_multi_stage_and_nothing_else() -> None:
    assert rules("") == ["DL001"]
    assert rules("# only a comment\n") == ["DL001"]


@pytest.mark.parametrize(
    "image", ["python", "python:latest", "ghcr.io/acme/base", "localhost:5000/x"]
)
def test_unpinned_base_images(image: str) -> None:
    findings = lint_dockerfile(
        GOOD.replace("FROM python:3.14-slim AS build", f"FROM {image} AS build")
    )
    assert [(f.rule, f.line) for f in findings] == [("DL002", 2)]


@pytest.mark.parametrize(
    "image",
    ["python:3.14-slim", "python@sha256:abc123", "localhost:5000/x:1.2", "$BASE_IMAGE", "scratch"],
)
def test_pinned_images_are_fine(image: str) -> None:
    assert (
        lint_dockerfile(GOOD.replace("FROM python:3.14-slim AS build", f"FROM {image} AS build"))
        == []
    )


def test_stage_references_are_not_base_images() -> None:
    text = GOOD.replace("FROM python:3.14-slim\n", "FROM build\n")
    assert lint_dockerfile(text) == []


def test_platform_flag_is_skipped_when_reading_the_image() -> None:
    text = GOOD.replace(
        "FROM python:3.14-slim AS build", "FROM --platform=linux/amd64 python:3.14-slim AS build"
    )
    assert lint_dockerfile(text) == []


@pytest.mark.parametrize("user_line", ["USER root", "USER 0", "USER root:root", "USER 0:0", ""])
def test_final_stage_must_not_run_as_root(user_line: str) -> None:
    findings = lint_dockerfile(GOOD.replace("USER app", user_line))
    assert [(f.rule, f.line) for f in findings] == [("DL003", 9)]


@pytest.mark.parametrize("user_line", ["USER app:app", "USER 10001", "USER ${UID}", "user app"])
def test_non_root_users_pass(user_line: str) -> None:
    assert lint_dockerfile(GOOD.replace("USER app", user_line)) == []


def test_user_in_an_earlier_stage_does_not_count() -> None:
    text = GOOD.replace("USER app\n", "").replace(
        "RUN python -m venv", "USER app\nRUN python -m venv"
    )
    assert rules(text) == ["DL003"]


def test_last_user_wins() -> None:
    assert rules(GOOD.replace("USER app\n", "USER app\nRUN true\nUSER root\n")) == ["DL003"]


def test_pip_install_needs_no_cache_dir() -> None:
    findings = lint_dockerfile(GOOD.replace("pip install --no-cache-dir", "pip install"))
    assert [(f.rule, f.line) for f in findings] == [("DL004", 6)]


def test_pip_no_cache_dir_can_come_from_the_environment() -> None:
    text = GOOD.replace(
        "FROM python:3.14-slim AS build\n",
        "FROM python:3.14-slim AS build\nENV PIP_NO_CACHE_DIR=1\n",
    )
    assert lint_dockerfile(text.replace("pip install --no-cache-dir", "pip install")) == []


def test_environment_setting_does_not_cross_stages() -> None:
    text = GOOD.replace(
        "FROM python:3.14-slim AS build\n",
        "FROM python:3.14-slim AS build\nENV PIP_NO_CACHE_DIR=1\n",
    )
    text = text.replace("RUN useradd --system app", "RUN python -m pip install tzdata")
    assert rules(text) == ["DL004"]


def test_copying_everything_before_installing_breaks_layer_caching() -> None:
    text = GOOD.replace("COPY requirements.txt .", "COPY . .")
    findings = lint_dockerfile(text)
    assert [(f.rule, f.line) for f in findings] == [("DL005", 5)]


def test_copy_from_another_stage_is_not_the_build_context() -> None:
    text = GOOD.replace("COPY requirements.txt .", "COPY --from=build . .")
    assert "DL005" not in rules(text)


def test_copy_everything_after_installing_is_fine() -> None:
    assert "DL005" not in rules(GOOD)


def test_missing_expose_and_healthcheck() -> None:
    text = GOOD.replace("EXPOSE 8080\n", "").replace(
        'HEALTHCHECK CMD ["python", "-c", "print(\'ok\')"]\n', ""
    )
    assert [(f.rule, f.line) for f in lint_dockerfile(text)] == [("DL006", 9), ("DL007", 9)]


def test_healthcheck_none_is_no_healthcheck() -> None:
    text = GOOD.replace('HEALTHCHECK CMD ["python", "-c", "print(\'ok\')"]', "HEALTHCHECK NONE")
    assert rules(text) == ["DL007"]


def test_expose_in_a_builder_stage_does_not_count() -> None:
    text = GOOD.replace("EXPOSE 8080\n", "").replace(
        "RUN python -m venv", "EXPOSE 8080\nRUN python -m venv"
    )
    assert rules(text) == ["DL006"]


@pytest.mark.parametrize(
    "line",
    ["CMD python service.py", "ENTRYPOINT python service.py", "CMD  uvicorn app:app --port 80"],
)
def test_shell_form_commands(line: str) -> None:
    text = GOOD.replace('CMD ["python", "service.py"]', line)
    assert [(f.rule, f.line) for f in lint_dockerfile(text)] == [("DL008", 16)]


def test_instructions_are_case_insensitive_and_comments_are_ignored() -> None:
    text = GOOD.replace("USER app", "# USER root\nuser app").replace("EXPOSE", "expose")
    assert lint_dockerfile(text) == []


def test_continuation_lines_are_joined_and_report_the_first_line() -> None:
    text = GOOD.replace(
        "RUN pip install --no-cache-dir -r requirements.txt",
        "RUN pip install \\\n    # a comment inside\n    -r requirements.txt",
    )
    findings = lint_dockerfile(text)
    assert [(f.rule, f.line) for f in findings] == [("DL004", 6)]


def test_continued_healthcheck_is_one_instruction() -> None:
    text = GOOD.replace(
        'HEALTHCHECK CMD ["python", "-c", "print(\'ok\')"]',
        'HEALTHCHECK --interval=30s \\\n  CMD ["python", "-c", "print(1)"]',
    )
    assert lint_dockerfile(text) == []


def test_findings_are_sorted_by_line_then_rule() -> None:
    bad = "FROM python\nCOPY . .\nRUN pip install -r requirements.txt\nCMD python app.py\n"
    findings = lint_dockerfile(bad)
    assert [(f.line, f.rule) for f in findings] == sorted((f.line, f.rule) for f in findings)
    assert {f.rule for f in findings} == {
        "DL001", "DL002", "DL003", "DL004", "DL005", "DL006", "DL007", "DL008",
    }  # fmt: skip


def test_dockerignore_requires_the_usual_entries() -> None:
    findings = lint_dockerignore("# comment\n.git\n*.md\n")
    assert [f.rule for f in findings] == ["DI001"] * 3
    assert [f.line for f in findings] == [1, 1, 1]
    messages = " ".join(f.message for f in findings)
    assert ".venv" in messages
    assert ".env" in messages
    assert "__pycache__" in messages
    assert ".git'" not in messages


def test_dockerignore_accepts_recursive_patterns() -> None:
    assert lint_dockerignore(".git\n.venv\n.env\n**/__pycache__\n") == []
    assert lint_dockerignore("  .git  \n.venv\n.env\n__pycache__\n") == []


def test_dockerignore_comments_do_not_count() -> None:
    assert len(lint_dockerignore("#.git\n#.venv\n#.env\n#__pycache__\n")) == 4
