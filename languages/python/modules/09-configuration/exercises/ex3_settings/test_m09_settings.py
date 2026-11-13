import json
import os
from pathlib import Path
from typing import Any

import pytest
from m09_settings import UploadSettings
from pydantic import ValidationError
from pydantic_settings import SettingsConfigDict

KEY = "k" * 16


def build(cls: type[UploadSettings] = UploadSettings, **init: Any) -> UploadSettings:
    # mypy (without the pydantic plugin) cannot see that sources fill in the fields
    return cls(**init)


def layered(toml: Path | None = None, env_file: Path | None = None) -> type[UploadSettings]:
    class Layered(UploadSettings):
        model_config = SettingsConfigDict(toml_file=toml, env_file=env_file)

    return Layered


@pytest.fixture(autouse=True)
def clean_environment(monkeypatch: pytest.MonkeyPatch) -> None:
    for name in list(os.environ):
        if name.startswith("UP_"):
            monkeypatch.delenv(name)


def test_defaults_apply_when_only_the_required_key_is_set(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("UP_SIGNING_KEY", KEY)

    settings = build()

    assert settings.upload_dir == Path("uploads")
    assert settings.max_upload_mb == 5
    assert settings.max_upload_bytes == 5 * 1024 * 1024
    assert settings.allowed_types == ["image/png"]


def test_environment_strings_are_parsed_into_typed_values(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("UP_SIGNING_KEY", KEY)
    monkeypatch.setenv("UP_UPLOAD_DIR", "/data/files")
    monkeypatch.setenv("UP_MAX_UPLOAD_MB", "10")

    settings = build()

    assert settings.upload_dir == Path("/data/files")
    assert settings.max_upload_mb == 10
    assert settings.max_upload_bytes == 10 * 1024 * 1024


@pytest.mark.parametrize(
    ("raw", "expected"),
    [
        ("image/png,image/jpeg", ["image/png", "image/jpeg"]),
        (" image/png , image/jpeg ,, ", ["image/png", "image/jpeg"]),
        ("", []),
    ],
)
def test_allowed_types_come_from_a_comma_separated_variable(
    monkeypatch: pytest.MonkeyPatch, raw: str, expected: list[str]
) -> None:
    monkeypatch.setenv("UP_SIGNING_KEY", KEY)
    monkeypatch.setenv("UP_ALLOWED_TYPES", raw)

    assert build().allowed_types == expected


def test_signing_key_is_required() -> None:
    with pytest.raises(ValidationError) as info:
        build()

    assert [error["loc"] for error in info.value.errors()] == [("signing_key",)]


def test_short_signing_key_is_rejected(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("UP_SIGNING_KEY", "short")

    with pytest.raises(ValidationError):
        build()


@pytest.mark.parametrize("value", ["0", "101", "-3", "lots"])
def test_upload_limit_must_be_between_1_and_100(
    monkeypatch: pytest.MonkeyPatch, value: str
) -> None:
    monkeypatch.setenv("UP_SIGNING_KEY", KEY)
    monkeypatch.setenv("UP_MAX_UPLOAD_MB", value)

    with pytest.raises(ValidationError) as info:
        build()

    assert info.value.errors()[0]["loc"] == ("max_upload_mb",)


def test_the_secret_is_hidden_unless_explicitly_revealed(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("UP_SIGNING_KEY", "super-secret-signing-key")

    settings = build()

    assert settings.signing_key.get_secret_value() == "super-secret-signing-key"
    assert "super-secret" not in repr(settings)
    assert "super-secret" not in str(settings)
    summary = settings.redacted()
    assert "super-secret" not in json.dumps(summary)
    assert summary["max_upload_mb"] == 5
    assert "signing_key" in summary


def test_toml_is_below_dotenv_below_environment(
    tmp_path: Path, monkeypatch: pytest.MonkeyPatch
) -> None:
    toml = tmp_path / "config.toml"
    toml.write_text(
        'upload_dir = "/from-toml"\nmax_upload_mb = 20\nallowed_types = ["text/plain"]\n'
        'signing_key = "toml-key-toml-key-toml"\n',
        encoding="utf-8",
    )
    env_file = tmp_path / ".env"
    env_file.write_text("UP_UPLOAD_DIR=/from-dotenv\nUP_MAX_UPLOAD_MB=30\n", encoding="utf-8")
    monkeypatch.setenv("UP_UPLOAD_DIR", "/from-env")

    settings = build(layered(toml, env_file))

    assert settings.upload_dir == Path("/from-env")
    assert settings.max_upload_mb == 30
    assert settings.allowed_types == ["text/plain"]
    assert settings.signing_key.get_secret_value() == "toml-key-toml-key-toml"


def test_init_arguments_beat_the_environment(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.setenv("UP_SIGNING_KEY", KEY)
    monkeypatch.setenv("UP_MAX_UPLOAD_MB", "10")

    assert build(max_upload_mb=40).max_upload_mb == 40
