"""Unit tests with pytest: plain asserts, parametrize, pytest.raises.

Run: pytest modules/12-testing/examples/01_unit_tests.py -vv --no-header
"""

import pytest
from m12_tasks import ValidationError, normalize_tags, validate_title


def test_title_is_trimmed() -> None:
    assert validate_title("  Write docs  ") == "Write docs"


@pytest.mark.parametrize(
    "title",
    [
        pytest.param("", id="empty"),
        pytest.param("   ", id="blank"),
        pytest.param("x" * 201, id="201-chars"),
    ],
)
def test_invalid_titles_are_rejected(title: str) -> None:
    with pytest.raises(ValidationError, match="1-200 characters"):
        validate_title(title)


@pytest.mark.parametrize(
    ("raw", "expected"),
    [
        pytest.param([], [], id="empty"),
        pytest.param(["Docs", "P1"], ["docs", "p1"], id="lower-cased"),
        pytest.param([" docs ", "docs", "DOCS"], ["docs"], id="duplicates-after-cleanup"),
        pytest.param(["", "  ", "ok"], ["ok"], id="blanks-dropped"),
        pytest.param(["b", "a", "b"], ["b", "a"], id="order-kept"),
    ],
)
def test_normalize_tags(raw: list[str], expected: list[str]) -> None:
    assert normalize_tags(raw) == expected


def test_more_than_ten_tags_is_an_error() -> None:
    with pytest.raises(ValidationError, match="at most 10"):
        normalize_tags([f"t{i}" for i in range(11)])


if __name__ == "__main__":
    raise SystemExit(pytest.main([__file__, "-vv", "--no-header"]))
