from datetime import date

import pytest
from m04_csv_tasks import Task, tasks_from_csv, tasks_to_csv

SAMPLE = [
    Task(1, "Write docs", "todo", date(2030, 1, 31), ("docs", "p1")),
    Task(2, "No date, no tags", "done", None, ()),
]


def test_header_and_plain_rows() -> None:
    assert tasks_to_csv(SAMPLE) == (
        "id,title,status,due_date,tags\n"
        "1,Write docs,todo,2030-01-31,docs;p1\n"
        '2,"No date, no tags",done,,\n'
    )


def test_empty_list_still_has_a_header() -> None:
    assert tasks_to_csv([]) == "id,title,status,due_date,tags\n"
    assert tasks_from_csv("id,title,status,due_date,tags\n") == []


def test_round_trip_with_awkward_titles() -> None:
    tasks = [
        Task(1, 'Say "hi", then leave', "todo", None, ("a",)),
        Task(2, "two\nlines", "in_progress", date(2031, 2, 3), ("x", "y", "z")),
        Task(3, "  padded  ", "todo", None, ()),
    ]
    assert tasks_from_csv(tasks_to_csv(tasks)) == tasks


def test_values_are_converted_back_to_types() -> None:
    [task] = tasks_from_csv("id,title,status,due_date,tags\n7,x,todo,2030-05-06,a;b\n")
    assert task == Task(7, "x", "todo", date(2030, 5, 6), ("a", "b"))
    assert isinstance(task.id, int)


def test_extra_columns_and_column_order_are_tolerated() -> None:
    text = "tags,extra,id,status,title,due_date\n,ignored,1,todo,x,\n"
    assert tasks_from_csv(text) == [Task(1, "x", "todo", None, ())]


def test_missing_column_names_the_column() -> None:
    with pytest.raises(ValueError, match="due_date"):
        tasks_from_csv("id,title,status,tags\n1,x,todo,\n")


@pytest.mark.parametrize(
    "row",
    ["abc,x,todo,,", "1,x,todo,2030-13-40,"],
)
def test_invalid_values_raise_value_error(row: str) -> None:
    with pytest.raises(ValueError, match=r"."):
        tasks_from_csv("id,title,status,due_date,tags\n" + row + "\n")
