import pytest
from m14_flags import ConfigError, RuntimeConfig, parse_runtime_config, uvicorn_args


def test_defaults() -> None:
    assert parse_runtime_config([], {}) == RuntimeConfig(
        host="127.0.0.1", port=8080, workers=1, log_level="info", env="development", reload=False
    )


def test_environment_overrides_defaults() -> None:
    env = {"ADDR": "0.0.0.0:9000", "WORKERS": "3", "LOG_LEVEL": "WARNING", "APP_ENV": "test"}
    config = parse_runtime_config([], env)
    assert (config.host, config.port, config.workers) == ("0.0.0.0", 9000, 3)
    assert (config.log_level, config.env) == ("warning", "test")


def test_flags_override_environment() -> None:
    env = {"ADDR": "0.0.0.0:9000", "WORKERS": "3", "LOG_LEVEL": "error", "APP_ENV": "test"}
    argv = [
        "--addr",
        "127.0.0.1:7000",
        "--workers",
        "2",
        "--log-level",
        "Debug",
        "--env",
        "development",
    ]
    config = parse_runtime_config(argv, env)
    assert config == RuntimeConfig("127.0.0.1", 7000, 2, "debug", "development", False)


def test_flag_syntax_with_equals() -> None:
    assert parse_runtime_config(["--addr=:9001"], {}).port == 9001


def test_empty_host_means_loopback() -> None:
    assert parse_runtime_config(["--addr", ":9001"], {}).host == "127.0.0.1"


@pytest.mark.parametrize(("cpus", "expected"), [(1, 1), (2, 2), (4, 4), (16, 4)])
def test_production_defaults_to_cpu_count_capped_at_four(cpus: int, expected: int) -> None:
    config = parse_runtime_config([], {"APP_ENV": "production"}, cpu_count=cpus)
    assert config.workers == expected


def test_explicit_workers_win_in_production() -> None:
    config = parse_runtime_config(["--workers", "8"], {"APP_ENV": "production"}, cpu_count=2)
    assert config.workers == 8


def test_development_defaults_to_one_worker_regardless_of_cpus() -> None:
    assert parse_runtime_config([], {}, cpu_count=16).workers == 1


@pytest.mark.parametrize(
    ("argv", "env", "message"),
    [
        (["--addr", "localhost"], {}, "(?i)addr"),
        (["--addr", "localhost:http"], {}, "(?i)addr"),
        (["--addr", "localhost:0"], {}, "(?i)addr"),
        (["--addr", "localhost:65536"], {}, "(?i)addr"),
        (["--workers", "0"], {}, "(?i)workers"),
        (["--workers", "two"], {}, "(?i)workers"),
        ([], {"WORKERS": "-1"}, "WORKERS"),
        ([], {"ADDR": "nope"}, "ADDR"),
        (["--log-level", "trace"], {}, "(?i)log"),
        ([], {"LOG_LEVEL": "verbose"}, "LOG_LEVEL"),
        (["--env", "staging"], {}, "(?i)env"),
        ([], {"APP_ENV": "prod"}, "APP_ENV"),
    ],
)
def test_invalid_values_raise_config_error(
    argv: list[str], env: dict[str, str], message: str
) -> None:
    with pytest.raises(ConfigError, match=message):
        parse_runtime_config(argv, env)


@pytest.mark.parametrize("argv", [["--bogus"], ["--work", "2"], ["--workers"], ["extra"]])
def test_argparse_problems_become_config_errors(argv: list[str]) -> None:
    with pytest.raises(ConfigError):
        parse_runtime_config(argv, {})


def test_reload_is_allowed_in_development() -> None:
    assert parse_runtime_config(["--reload"], {}).reload is True


def test_reload_is_refused_in_production() -> None:
    with pytest.raises(ConfigError, match="production"):
        parse_runtime_config(["--reload"], {"APP_ENV": "production"})
    with pytest.raises(ConfigError, match="production"):
        parse_runtime_config(["--reload", "--env", "production"], {})


def test_reload_is_refused_with_multiple_workers() -> None:
    with pytest.raises(ConfigError, match="worker"):
        parse_runtime_config(["--reload", "--workers", "2"], {})


def test_uvicorn_args() -> None:
    config = RuntimeConfig("0.0.0.0", 8080, 2, "info", "production", False)
    assert uvicorn_args(config) == [
        "uvicorn", "task_tracker.app:app", "--host", "0.0.0.0", "--port", "8080",
        "--workers", "2", "--log-level", "info",
    ]  # fmt: skip


def test_uvicorn_args_with_reload() -> None:
    config = RuntimeConfig("127.0.0.1", 8000, 1, "debug", "development", True)
    assert uvicorn_args(config)[-1] == "--reload"
    assert "--reload" not in uvicorn_args(RuntimeConfig("h", 1, 1, "info", "test", False))
