"""Exercise 3 (Stretch): typed settings for the file service with pydantic-settings."""

from pathlib import Path

from pydantic import SecretStr
from pydantic_settings import BaseSettings, SettingsConfigDict


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
    max_upload_mb: int = 5
    allowed_types: list[str] = ["image/png"]
    signing_key: SecretStr

    @property
    def max_upload_bytes(self) -> int:
        """max_upload_mb in bytes (1 MB = 1024 * 1024 bytes)."""
        raise NotImplementedError("TODO: implement max_upload_bytes")

    def redacted(self) -> dict[str, object]:
        """A JSON-serialisable dict of all settings that never contains the signing key's
        value (suitable for logging at startup)."""
        raise NotImplementedError("TODO: implement redacted")
