from collections.abc import Mapping, Sequence

import pytest
from m09_layered import Config, ConfigError, load_config


def load(
    file_values: Mapping[str, object] | None = None,
    env: Mapping[str, str] | None = None,
    argv: Sequence[str] = (),
) -> Config:
    return load_config(file_values=file_values or {}, env=env or {}, argv=argv)


def test_defaults_when_nothing_is_given() -> None:
    assert load() == Config("127.0.0.1", 8080, "info", False)


def test_file_overrides_defaults_key_by_key() -> None:
    config = load(file_values={"port": 9000, "debug": True})

    assert config == Config("127.0.0.1", 9000, "info", True)


def test_env_overrides_file_and_converts_types() -> None:
    config = load(
        file_values={"port": 9000, "log_level": "warn"},
        env={"APP_PORT": "9100", "APP_DEBUG": "Yes", "APP_HOST": "0.0.0.0"},
    )

    assert config == Config("0.0.0.0", 9100, "warn", True)


def test_command_line_overrides_everything() -> None:
    config = load(
        file_values={"port": 9000, "debug": True},
        env={"APP_PORT": "9100", "APP_LOG_LEVEL": "warn"},
        argv=["--port", "9200", "--no-debug", "--log-level", "debug"],
    )

    assert config == Config("127.0.0.1", 9200, "debug", False)


def test_flags_that_were_not_given_do_not_override_lower_layers() -> None:
    config = load(env={"APP_PORT": "9100", "APP_DEBUG": "true"}, argv=["--host", "example.test"])

    assert (config.host, config.port, config.debug) == ("example.test", 9100, True)


def test_all_problems_are_reported_together() -> None:
    with pytest.raises(ConfigError) as info:
        load(env={"APP_PORT": "abc", "APP_LOG_LEVEL": "loud", "APP_DEBUG": "maybe"})

    errors = info.value.errors
    assert len(errors) == 3
    assert any("port" in e for e in errors)
    assert any("log_level" in e for e in errors)
    assert any("debug" in e for e in errors)


@pytest.mark.parametrize("port", ["0", "65536", "-1"])
def test_port_range_is_enforced(port: str) -> None:
    with pytest.raises(ConfigError) as info:
        load(argv=["--port", port])

    assert len(info.value.errors) == 1
    assert "port" in info.value.errors[0]


def test_only_the_winning_value_is_validated() -> None:
    config = load(file_values={"port": "not a number"}, env={"APP_PORT": "9000"})

    assert config.port == 9000


def test_unknown_file_keys_are_errors() -> None:
    with pytest.raises(ConfigError) as info:
        load(file_values={"colour": "blue", "port": 9000})

    assert info.value.errors == ["unknown key 'colour' in config file"]


def test_invalid_host_from_the_file() -> None:
    with pytest.raises(ConfigError, match="host"):
        load(file_values={"host": "  "})


def test_command_line_mistakes_raise_config_error_not_system_exit() -> None:
    with pytest.raises(ConfigError):
        load(argv=["--bogus"])
    with pytest.raises(ConfigError):
        load(argv=["--port"])
