import json
from datetime import UTC, date, datetime, timedelta, timezone
from decimal import Decimal

import pytest
from m04_json_encoder import encode_json


def test_compact_sorted_and_not_ascii_escaped() -> None:
    assert encode_json({"b": 1, "a": ["é", None, True]}) == '{"a":["é",null,true],"b":1}'


def test_aware_datetime_is_converted_to_utc_seconds() -> None:
    plus_one = timezone(timedelta(hours=1))
    value = datetime(2030, 1, 1, 10, 30, 15, 123456, tzinfo=plus_one)
    assert encode_json({"at": value}) == '{"at":"2030-01-01T09:30:15Z"}'
    assert encode_json(datetime(2030, 1, 1, tzinfo=UTC)) == '"2030-01-01T00:00:00Z"'


def test_naive_datetime_is_rejected() -> None:
    with pytest.raises(ValueError, match="naive"):
        encode_json(datetime(2030, 1, 1, 9, 0))


def test_date_and_decimal() -> None:
    assert encode_json({"due": date(2030, 1, 31), "hours": Decimal("1.10")}) == (
        '{"due":"2030-01-31","hours":"1.10"}'
    )


def test_sets_become_sorted_lists_even_when_nested() -> None:
    assert encode_json({"tags": {"b", "a", "c"}, "n": frozenset({2, 1})}) == (
        '{"n":[1,2],"tags":["a","b","c"]}'
    )


def test_unknown_types_raise_type_error() -> None:
    with pytest.raises(TypeError):
        encode_json({"x": object()})


@pytest.mark.parametrize("bad", [float("nan"), float("inf")])
def test_non_finite_numbers_are_rejected(bad: float) -> None:
    with pytest.raises(ValueError, match=r"."):
        encode_json(bad)


def test_output_is_valid_json_for_a_round_trip() -> None:
    text = encode_json({"when": datetime(2030, 1, 1, tzinfo=UTC), "tags": {"x"}})
    assert json.loads(text) == {"when": "2030-01-01T00:00:00Z", "tags": ["x"]}
