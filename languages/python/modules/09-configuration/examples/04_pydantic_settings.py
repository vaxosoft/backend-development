"""Typed settings with pydantic-settings: env prefix, secrets, validation and layers."""

import os
from pathlib import Path
from tempfile import TemporaryDirectory
from unittest import mock

from m09_task_settings import Settings
from pydantic import ValidationError
from pydantic_settings import SettingsConfigDict


def describe(settings: Settings) -> None:
    print(
        f"port={settings.port} log_level={settings.log_level} debug={settings.debug} "
        f"database_url={settings.database_url}"
    )


def main() -> None:
    # Without the pydantic mypy plugin, mypy thinks every field is a required argument, hence
    # the `type: ignore[call-arg]` on each Settings(...) call: the values come from the sources.
    # 1. Environment variables are strings; the field types do the parsing
    env = {"TT_PORT": "9000", "TT_LOG_LEVEL": "debug", "TT_DEBUG": "yes", "TT_API_TOKEN": "s3cr3t"}
    with mock.patch.dict(os.environ, env, clear=True):
        settings = Settings()  # type: ignore[call-arg]
    describe(settings)
    print("repr hides the secret:", repr(settings.api_token))
    print("explicit reveal:", settings.api_token.get_secret_value())

    # 2. Invalid configuration fails at startup, and reports every problem at once
    bad = {"TT_PORT": "70000", "TT_LOG_LEVEL": "loud"}
    with mock.patch.dict(os.environ, bad, clear=True):
        try:
            Settings()  # type: ignore[call-arg]
        except ValidationError as exc:
            for error in exc.errors():
                print(f"TT_{str(error['loc'][0]).upper()}: {error['msg']}")

    # 3. Layers: init arguments > environment > .env file > TOML file > defaults
    with TemporaryDirectory() as tmp:
        toml_path = Path(tmp) / "config.toml"
        toml_path.write_text(
            'port = 7000\nlog_level = "warn"\ndatabase_url = "sqlite:///from-toml.db"\n',
            encoding="utf-8",
        )
        dotenv_path = Path(tmp) / ".env"
        dotenv_path.write_text("TT_PORT=7100\nTT_DEBUG=true\n", encoding="utf-8")

        class LayeredSettings(Settings):
            model_config = SettingsConfigDict(toml_file=toml_path)

        with mock.patch.dict(
            os.environ, {"TT_LOG_LEVEL": "error", "TT_API_TOKEN": "t"}, clear=True
        ):
            layered = LayeredSettings(_env_file=dotenv_path)  # type: ignore[call-arg]
            describe(layered)
            describe(LayeredSettings(_env_file=dotenv_path, port=7300))  # type: ignore[call-arg]


if __name__ == "__main__":
    main()
