"""Exercise 3 (Stretch): typed settings for the file service with pydantic-settings."""

from pathlib import Path
from typing import Annotated

from pydantic import Field, SecretStr, field_validator
from pydantic_settings import (
    BaseSettings,
    NoDecode,
    PydanticBaseSettingsSource,
    SettingsConfigDict,
    TomlConfigSettingsSource,
)


class UploadSettings(BaseSettings):
    """Read from UP_<NAME> environment variables (and the other layers, see the tests).

    - upload_dir: Path, default "uploads"
    - max_upload_mb: int from 1 to 100, default 5
    - allowed_types: list of MIME types, default ["image/png"]. In the environment it is
      a comma-separated string: UP_ALLOWED_TYPES="image/png, image/jpeg" (strip items,
      drop empty ones). pydantic-settings would try to decode it as JSON by default.
    - signing_key: SecretStr, required, at least 16 characters
    - sources, highest priority first: init arguments, environment, .env file, TOML file
      (`toml_file` of the model config; use TomlConfigSettingsSource), field defaults
    """

    model_config = SettingsConfigDict(env_prefix="UP_")

    upload_dir: Path = Path("uploads")
    max_upload_mb: int = Field(5, ge=1, le=100)
    allowed_types: Annotated[list[str], NoDecode] = ["image/png"]
    signing_key: SecretStr = Field(min_length=16)

    @field_validator("allowed_types", mode="before")
    @classmethod
    def split_csv(cls, value: object) -> object:
        if isinstance(value, str):
            return [item.strip() for item in value.split(",") if item.strip()]
        return value

    @classmethod
    def settings_customise_sources(
        cls,
        settings_cls: type[BaseSettings],
        init_settings: PydanticBaseSettingsSource,
        env_settings: PydanticBaseSettingsSource,
        dotenv_settings: PydanticBaseSettingsSource,
        file_secret_settings: PydanticBaseSettingsSource,
    ) -> tuple[PydanticBaseSettingsSource, ...]:
        return (
            init_settings,
            env_settings,
            dotenv_settings,
            TomlConfigSettingsSource(settings_cls),
        )

    @property
    def max_upload_bytes(self) -> int:
        """max_upload_mb in bytes (1 MB = 1024 * 1024 bytes)."""
        return self.max_upload_mb * 1024 * 1024

    def redacted(self) -> dict[str, object]:
        """A JSON-serialisable dict of all settings that never contains the signing key's
        value (suitable for logging at startup)."""
        return self.model_dump(mode="json")  # JSON mode masks SecretStr values
