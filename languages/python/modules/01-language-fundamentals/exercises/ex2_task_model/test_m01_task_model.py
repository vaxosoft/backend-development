from datetime import date

import pytest
from m01_task_model import Status, Task, ValidationError, parse_task


def test_minimal_input_uses_defaults() -> None:
    task = parse_task(7, {"title": "  Write docs "})
    assert task == Task(7, "Write docs", Status.TODO, None, (), None)


def test_full_input() -> None:
    task = parse_task(
        1,
        {
            "title": "Ship",
            "status": "in_progress",
            "due_date": "2030-01-31",
            "tags": ["docs", "p1"],
            "owner_id": 3,
        },
    )
    assert task.status is Status.IN_PROGRESS
    assert task.due_date == date(2030, 1, 31)
    assert task.tags == ("docs", "p1")
    assert task.owner_id == 3


def test_to_dict_matches_wire_format() -> None:
    task = parse_task(
        1, {"title": "Ship", "status": "done", "due_date": "2030-01-31", "tags": ["a"]}
    )
    assert task.to_dict() == {
        "id": 1,
        "title": "Ship",
        "status": "done",
        "due_date": "2030-01-31",
        "tags": ["a"],
        "owner_id": None,
    }


def test_task_is_immutable() -> None:
    task = parse_task(1, {"title": "x"})
    with pytest.raises(AttributeError):
        task.title = "y"  # type: ignore[misc]


@pytest.mark.parametrize(
    ("data", "field"),
    [
        ({}, "title"),
        ({"title": "   "}, "title"),
        ({"title": "x" * 201}, "title"),
        ({"title": 5}, "title"),
        ({"title": "a", "status": "finished"}, "status"),
        ({"title": "a", "due_date": "31/01/2030"}, "due_date"),
        ({"title": "a", "due_date": 20300131}, "due_date"),
        ({"title": "a", "tags": ["Upper"]}, "tags"),
        ({"title": "a", "tags": ["dup", "dup"]}, "tags"),
        ({"title": "a", "tags": [f"t{i}" for i in range(11)]}, "tags"),
        ({"title": "a", "tags": ["x" * 31]}, "tags"),
        ({"title": "a", "tags": "docs"}, "tags"),
        ({"title": "a", "owner_id": "3"}, "owner_id"),
        ({"title": "a", "owner_id": True}, "owner_id"),
    ],
)
def test_invalid_field_is_reported(data: dict[str, object], field: str) -> None:
    with pytest.raises(ValidationError) as info:
        parse_task(1, data)
    assert set(info.value.fields) == {field}


def test_all_problems_are_collected() -> None:
    with pytest.raises(ValidationError) as info:
        parse_task(1, {"title": "", "status": "x", "tags": ["A"]})
    assert set(info.value.fields) == {"title", "status", "tags"}


def test_title_is_stripped_before_length_check() -> None:
    assert parse_task(1, {"title": " " + "x" * 200 + " "}).title == "x" * 200


def test_null_optionals_are_accepted() -> None:
    task = parse_task(1, {"title": "a", "due_date": None, "owner_id": None})
    assert task.due_date is None
    assert task.owner_id is None
