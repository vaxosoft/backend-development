import pytest
from m14_buildinfo import BuildInfo, load_build_info, parse_version, read_project_version


def test_defaults_when_nothing_is_injected() -> None:
    info = load_build_info({})
    assert info == BuildInfo(version="dev", commit="unknown")
    assert info.user_agent() == "task-tracker/dev"


def test_values_come_from_the_environment() -> None:
    info = load_build_info({"APP_VERSION": "1.4.2", "GIT_COMMIT": "0123456789abcdef"})
    assert info.version == "1.4.2"
    assert info.short_commit == "0123456"
    assert info.user_agent() == "task-tracker/1.4.2"
    assert info.as_dict() == {"version": "1.4.2", "commit": "0123456"}


@pytest.mark.parametrize("blank", ["", "   ", "\n"])
def test_blank_values_count_as_unset(blank: str) -> None:
    assert load_build_info({"APP_VERSION": blank, "GIT_COMMIT": blank}) == BuildInfo(
        "dev", "unknown"
    )


def test_values_are_stripped() -> None:
    assert load_build_info({"APP_VERSION": " 2.0.0\n"}).version == "2.0.0"


def test_short_commit_keeps_short_values() -> None:
    assert BuildInfo("1.0.0", "abc").short_commit == "abc"
    assert BuildInfo("1.0.0", "unknown").short_commit == "unknown"


def test_read_project_version() -> None:
    text = '[project]\nname = "task-tracker"\nversion = "1.4.2"\n'
    assert read_project_version(text) == "1.4.2"


@pytest.mark.parametrize(
    "text",
    [
        '[project]\nname = "x"\n',
        '[project]\nname = "x"\ndynamic = ["version"]\n',
        "[tool.ruff]\nline-length = 100\n",
        "",
    ],
)
def test_read_project_version_requires_a_static_version(text: str) -> None:
    with pytest.raises(ValueError, match="static"):
        read_project_version(text)


def test_parse_version() -> None:
    assert parse_version("1.4.2") == (1, 4, 2)
    assert parse_version("v10.0.31") == (10, 0, 31)
    assert parse_version("1.10.0") > parse_version("1.9.9")


@pytest.mark.parametrize(
    "text", ["1.4", "1.4.2.1", "1.4.x", "v", "", "1.4.2-rc1", " 1.4.2", "V1.4.2"]
)
def test_parse_version_rejects_other_shapes(text: str) -> None:
    with pytest.raises(ValueError, match="MAJOR"):
        parse_version(text)
