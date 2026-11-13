from typing import Any

import pytest
from m04_task_patch import PatchError, apply_patch


@pytest.fixture
def task() -> dict[str, Any]:
    return {
        "id": 1,
        "title": "Write docs",
        "status": "todo",
        "due_date": "2030-01-31",
        "tags": ["docs"],
        "owner_id": 5,
        "created_at": "2030-01-01T09:00:00Z",
    }


def test_only_present_keys_change(task: dict[str, Any]) -> None:
    patched = apply_patch(task, '{"status": "done", "tags": ["a", "b"]}')
    assert patched == {**task, "status": "done", "tags": ["a", "b"]}


def test_empty_object_is_a_no_op_copy(task: dict[str, Any]) -> None:
    patched = apply_patch(task, "{}")
    assert patched == task
    assert patched is not task


def test_input_is_not_mutated(task: dict[str, Any]) -> None:
    before = {**task, "tags": list(task["tags"])}
    apply_patch(task, '{"title": "New", "tags": ["x"]}')
    assert task == before


def test_null_clears_nullable_fields(task: dict[str, Any]) -> None:
    patched = apply_patch(task, '{"due_date": null, "owner_id": null}')
    assert patched["due_date"] is None
    assert patched["owner_id"] is None
    assert patched["title"] == "Write docs"


def test_values_are_stored_in_wire_form(task: dict[str, Any]) -> None:
    patched = apply_patch(task, '{"due_date": "2031-02-03", "status": "in_progress"}')
    assert patched["due_date"] == "2031-02-03"
    assert type(patched["status"]) is str
    assert patched["status"] == "in_progress"


def test_title_is_trimmed(task: dict[str, Any]) -> None:
    assert apply_patch(task, '{"title": "  Spaced  "}')["title"] == "Spaced"


@pytest.mark.parametrize("field", ["title", "status", "tags"])
def test_null_is_rejected_for_required_fields(task: dict[str, Any], field: str) -> None:
    with pytest.raises(PatchError) as info:
        apply_patch(task, f'{{"{field}": null}}')
    assert set(info.value.fields) == {field}


@pytest.mark.parametrize(
    ("body", "field"),
    [
        ('{"title": "   "}', "title"),
        ('{"title": "' + "x" * 201 + '"}', "title"),
        ('{"status": "blocked"}', "status"),
        ('{"due_date": "2030-13-40"}', "due_date"),
        ('{"tags": ["a", "a"]}', "tags"),
        ('{"owner_id": "abc"}', "owner_id"),
        ('{"id": 99}', "id"),
        ('{"colour": "red"}', "colour"),
    ],
)
def test_invalid_values_and_unknown_keys(task: dict[str, Any], body: str, field: str) -> None:
    with pytest.raises(PatchError) as info:
        apply_patch(task, body)
    assert set(info.value.fields) == {field}
    assert all(isinstance(msg, str) and msg for msg in info.value.fields.values())


def test_every_problem_is_reported_at_once(task: dict[str, Any]) -> None:
    with pytest.raises(PatchError) as info:
        apply_patch(task, '{"title": "", "status": "nope"}')
    assert set(info.value.fields) == {"title", "status"}


@pytest.mark.parametrize("body", ['{"title": "x",}', "", "[1]", "null"])
def test_malformed_or_non_object_bodies(task: dict[str, Any], body: str) -> None:
    with pytest.raises(PatchError) as info:
        apply_patch(task, body)
    assert "body" in info.value.fields
