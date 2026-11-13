"""Typed settings for the Task Tracker service, shared by examples 04 and 05."""

from typing import Literal

from pydantic import Field, SecretStr
from pydantic_settings import (
    BaseSettings,
    PydanticBaseSettingsSource,
    SettingsConfigDict,
    TomlConfigSettingsSource,
)


class Settings(BaseSettings):
    """Every field can come from TT_<NAME> in the environment, a .env file or a TOML file."""

    model_config = SettingsConfigDict(env_prefix="TT_")

    port: int = Field(8080, ge=1, le=65535)
    log_level: Literal["debug", "info", "warn", "error"] = "info"
    database_url: str = "sqlite:///tasks.db"
    api_token: SecretStr  # required: no default, so a missing value fails at startup
    debug: bool = False

    @classmethod
    def settings_customise_sources(
        cls,
        settings_cls: type[BaseSettings],
        init_settings: PydanticBaseSettingsSource,
        env_settings: PydanticBaseSettingsSource,
        dotenv_settings: PydanticBaseSettingsSource,
        file_secret_settings: PydanticBaseSettingsSource,
    ) -> tuple[PydanticBaseSettingsSource, ...]:
        # Earlier sources win: arguments > environment > .env > TOML file > field defaults
        return (
            init_settings,
            env_settings,
            dotenv_settings,
            TomlConfigSettingsSource(settings_cls),
        )
