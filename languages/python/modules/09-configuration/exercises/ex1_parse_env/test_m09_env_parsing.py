import pytest
from m09_env_parsing import ConfigError, parse_bool, parse_csv, parse_int, require


def test_require_returns_the_value() -> None:
    assert require({"TT_TOKEN": "abc"}, "TT_TOKEN") == "abc"


@pytest.mark.parametrize("env", [{}, {"TT_TOKEN": ""}, {"TT_TOKEN": "   "}])
def test_require_rejects_missing_and_blank_values(env: dict[str, str]) -> None:
    with pytest.raises(ConfigError, match="TT_TOKEN is required"):
        require(env, "TT_TOKEN")


@pytest.mark.parametrize("raw", ["1", "true", "TRUE", " yes ", "On"])
def test_truthy_words(raw: str) -> None:
    assert parse_bool("TT_DEBUG", raw) is True


@pytest.mark.parametrize("raw", ["0", "false", "False", " no ", "OFF"])
def test_falsy_words(raw: str) -> None:
    assert parse_bool("TT_DEBUG", raw) is False


@pytest.mark.parametrize("raw", ["", "maybe", "2", "tru"])
def test_unknown_boolean_words_are_errors_not_silently_true(raw: str) -> None:
    with pytest.raises(ConfigError, match="TT_DEBUG"):
        parse_bool("TT_DEBUG", raw)


def test_parse_int_accepts_whitespace_and_negative_numbers() -> None:
    assert parse_int("TT_PORT", " 8080 ") == 8080
    assert parse_int("TT_OFFSET", "-5") == -5


@pytest.mark.parametrize("raw", ["", "abc", "1.5", "10 000"])
def test_parse_int_wraps_conversion_errors_in_config_error(raw: str) -> None:
    with pytest.raises(ConfigError, match="TT_PORT"):
        parse_int("TT_PORT", raw)


def test_parse_int_bounds_are_inclusive() -> None:
    assert parse_int("TT_PORT", "1", minimum=1, maximum=65535) == 1
    assert parse_int("TT_PORT", "65535", minimum=1, maximum=65535) == 65535
    with pytest.raises(ConfigError, match="TT_PORT"):
        parse_int("TT_PORT", "0", minimum=1, maximum=65535)
    with pytest.raises(ConfigError, match="TT_PORT"):
        parse_int("TT_PORT", "65536", minimum=1, maximum=65535)


@pytest.mark.parametrize(
    ("raw", "expected"),
    [
        ("a,b,c", ["a", "b", "c"]),
        ("a, b,,c ", ["a", "b", "c"]),
        ("", []),
        (" , ", []),
        ("x", ["x"]),
    ],
)
def test_parse_csv(raw: str, expected: list[str]) -> None:
    assert parse_csv(raw) == expected
