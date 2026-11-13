"""Configuration tests: load_config is a pure function of a mapping, so no real environment."""

import logging

import pytest
from p7_tested_service_docker_ci.config import Config, ConfigError, load_config


def test_empty_environment_gives_the_defaults() -> None:
    assert load_config({}) == Config(host="127.0.0.1", port=8080, log_level="info", env="dev")


def test_valid_values_are_parsed_and_log_level_is_case_insensitive() -> None:
    config = load_config({"PORT": "9000", "LOG_LEVEL": "WARN", "ENV": "prod", "HOST": "0.0.0.0"})

    assert config == Config(host="0.0.0.0", port=9000, log_level="warn", env="prod")
    assert config.logging_level == logging.WARNING


@pytest.mark.parametrize(
    ("variable", "value"),
    [
        ("PORT", "abc"),
        ("PORT", "0"),
        ("PORT", "65536"),
        ("PORT", ""),
        ("LOG_LEVEL", "loud"),
        ("ENV", "staging"),
        ("HOST", "  "),
    ],
)
def test_invalid_value_fails_with_a_message_naming_the_variable(variable: str, value: str) -> None:
    with pytest.raises(ConfigError) as excinfo:
        load_config({variable: value})

    assert variable in str(excinfo.value)
    assert repr(value) in str(excinfo.value)


def test_every_problem_is_reported_at_once() -> None:
    with pytest.raises(ConfigError) as excinfo:
        load_config({"PORT": "x", "LOG_LEVEL": "loud", "ENV": "nope"})

    message = str(excinfo.value)
    assert message.startswith("invalid configuration: ")
    assert all(name in message for name in ("PORT", "LOG_LEVEL", "ENV"))
